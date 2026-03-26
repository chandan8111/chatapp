package e2ee

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"time"

	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/pbkdf2"
)

const (
	keySize      = 32
	nonceSize    = 12
	chainKeySize = 32
	messageKeySize = 32
	saltSize     = 32
	infoString   = "Double Ratchet Chat"
)

var (
	ErrInvalidMessage      = errors.New("invalid message format")
	ErrInvalidSignature    = errors.New("invalid message signature")
	ErrChainKeyStale       = errors.New("chain key is stale")
	ErrMaxSkipMessages     = errors.New("maximum skip messages exceeded")
	ErrInvalidKeyExchange  = errors.New("invalid key exchange message")
)

type DoubleRatchet struct {
	// Identity keys
	identityKeyPair *ecdsa.PrivateKey
	
	// Root key and DH key pair
	rootKey       []byte
	dhKeyPair     *ecdsa.PrivateKey
	dhPublicKey   *ecdsa.PublicKey
	
	// Sending chain
	sendingChainKey []byte
	sendingCounter  uint32
	
	// Receiving chains
	receivingChains map[string]*ReceivingChain
	
	// Skip message keys
	skipMessageKeys map[string][]byte
	
	// Configuration
	maxSkipMessages int
}

type ReceivingChain struct {
	chainKey    []byte
	counter     uint32
	lastMessage *MessageKeys
}

type MessageKeys struct {
	messageKey []byte
	nonce      []byte
	counter    uint32
}

type EncryptedMessage struct {
	Version          int    `json:"version"`
	MessageID        string `json:"message_id"`
	DHPublicKey      []byte `json:"dh_public_key"`
	Counter          uint32 `json:"counter"`
	Ciphertext       []byte `json:"ciphertext"`
	AuthTag          []byte `json:"auth_tag"`
	PreviousChainKey []byte `json:"previous_chain_key,omitempty"`
}

type KeyExchangeMessage struct {
	Version       int    `json:"version"`
	MessageID     string `json:"message_id"`
	DHPublicKey   []byte `json:"dh_public_key"`
	IdentityKey   []byte `json:"identity_key"`
	SignedPreKey  []byte `json:"signed_prekey"`
	OneTimePreKey []byte `json:"one_time_prekey,omitempty"`
	Signature     []byte `json:"signature"`
}

type PreKeyBundle struct {
	RegistrationID  uint32
	DeviceID        uint32
	PreKeyID        uint32
	PreKey          *ecdsa.PublicKey
	SignedPreKeyID  uint32
	SignedPreKey    *ecdsa.PublicKey
	SignedPreKeySignature []byte
	IdentityKey     *ecdsa.PublicKey
}

type X3DHResult struct {
	sharedSecret []byte
	identityKey  *ecdsa.PrivateKey
}

func NewDoubleRatchet(identityKeyPair *ecdsa.PrivateKey) *DoubleRatchet {
	return &DoubleRatchet{
		identityKeyPair:   identityKeyPair,
		receivingChains:   make(map[string]*ReceivingChain),
		skipMessageKeys:   make(map[string][]byte),
		maxSkipMessages:   1000,
	}
}

func GenerateIdentityKeyPair() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
}

func GenerateEphemeralKeyPair() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
}

func GeneratePreKey() (*ecdsa.PrivateKey, uint32, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, 0, err
	}
	
	// Generate random pre-key ID
	preKeyID := uint32(time.Now().UnixNano() % 0xFFFFFFFF)
	
	return privateKey, preKeyID, nil
}

func (dr *DoubleRatchet) InitializeAsAlice(rootKey []byte, bobPublicKey *ecdsa.PublicKey) error {
	// Generate new DH key pair
	dhKeyPair, err := GenerateEphemeralKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate DH key pair: %w", err)
	}
	
	// Calculate new root key and chain key
	newRootKey, chainKey, err := dr.dhRatchet(rootKey, dhKeyPair, bobPublicKey)
	if err != nil {
		return fmt.Errorf("failed to perform DH ratchet: %w", err)
	}
	
	dr.rootKey = newRootKey
	dr.dhKeyPair = dhKeyPair
	dr.dhPublicKey = &dhKeyPair.PublicKey
	dr.sendingChainKey = chainKey
	dr.sendingCounter = 0
	
	return nil
}

func (dr *DoubleRatchet) InitializeAsBob(rootKey []byte, alicePublicKey *ecdsa.PublicKey) error {
	// Generate new DH key pair
	dhKeyPair, err := GenerateEphemeralKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate DH key pair: %w", err)
	}
	
	// Calculate new root key and chain key
	newRootKey, chainKey, err := dr.dhRatchet(rootKey, dhKeyPair, alicePublicKey)
	if err != nil {
		return fmt.Errorf("failed to perform DH ratchet: %w", err)
	}
	
	dr.rootKey = newRootKey
	dr.dhKeyPair = dhKeyPair
	dr.dhPublicKey = &dhKeyPair.PublicKey
	
	// Initialize receiving chain for Alice
	chainKeyBytes := make([]byte, chainKeySize)
	copy(chainKeyBytes, chainKey)
	
	dr.receivingChains[publicKeyToBytes(alicePublicKey)] = &ReceivingChain{
		chainKey: chainKeyBytes,
		counter:  0,
	}
	
	return nil
}

func (dr *DoubleRatchet) Encrypt(plaintext []byte) (*EncryptedMessage, error) {
	// Generate message keys from sending chain key
	messageKey, nonce, err := dr.generateMessageKeys(dr.sendingChainKey, dr.sendingCounter)
	if err != nil {
		return nil, fmt.Errorf("failed to generate message keys: %w", err)
	}
	
	// Encrypt message
	ciphertext, authTag, err := dr.encryptWithAEAD(messageKey, nonce, plaintext)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt message: %w", err)
	}
	
	// Update sending chain key
	newChainKey, err := dr.kdf(dr.sendingChainKey, []byte{0x01})
	if err != nil {
		return nil, fmt.Errorf("failed to update chain key: %w", err)
	}
	dr.sendingChainKey = newChainKey
	dr.sendingCounter++
	
	return &EncryptedMessage{
		Version:     1,
		MessageID:   generateMessageID(),
		DHPublicKey: publicKeyToBytes(dr.dhPublicKey),
		Counter:     dr.sendingCounter - 1,
		Ciphertext:  ciphertext,
		AuthTag:     authTag,
	}, nil
}

func (dr *DoubleRatchet) Decrypt(message *EncryptedMessage) ([]byte, error) {
	// Check if we need to perform DH ratchet
	dhPublicKey, err := parsePublicKey(message.DHPublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DH public key: %w", err)
	}
	
	publicKeyBytes := publicKeyToBytes(dhPublicKey)
	
	// Check if this is a new DH public key
	if _, exists := dr.receivingChains[publicKeyBytes]; !exists {
		// Perform DH ratchet
		newRootKey, receivingChainKey, err := dr.dhRatchet(dr.rootKey, dr.dhKeyPair, dhPublicKey)
		if err != nil {
			return nil, fmt.Errorf("failed to perform DH ratchet: %w", err)
		}
		
		// Move current sending chain to skip message keys
		if dr.sendingChainKey != nil {
			oldPublicKeyBytes := publicKeyToBytes(dr.dhPublicKey)
			dr.skipMessageKeys[oldPublicKeyBytes] = dr.sendingChainKey
		}
		
		// Update root key and generate new sending chain
		dr.rootKey = newRootKey
		newRootKey, sendingChainKey, err := dr.dhRatchet(dr.rootKey, dr.dhKeyPair, dhPublicKey)
		if err != nil {
			return nil, fmt.Errorf("failed to generate new sending chain: %w", err)
		}
		
		dr.rootKey = newRootKey
		dr.sendingChainKey = sendingChainKey
		dr.sendingCounter = 0
		
		// Add new receiving chain
		dr.receivingChains[publicKeyBytes] = &ReceivingChain{
			chainKey: receivingChainKey,
			counter:  0,
		}
	}
	
	// Get receiving chain
	receivingChain, exists := dr.receivingChains[publicKeyBytes]
	if !exists {
		return nil, ErrInvalidMessage
	}
	
	// Check if we need to skip messages
	if message.Counter < receivingChain.counter {
		return nil, ErrChainKeyStale
	}
	
	// Skip messages if necessary
	for receivingChain.counter < message.Counter {
		if len(dr.skipMessageKeys) >= dr.maxSkipMessages {
			return nil, ErrMaxSkipMessages
		}
		
		skipKey, err := dr.kdf(receivingChain.chainKey, []byte{0x01})
		if err != nil {
			return nil, fmt.Errorf("failed to generate skip key: %w", err)
		}
		
		skipKeyID := fmt.Sprintf("%s:%d", publicKeyBytes, receivingChain.counter)
		dr.skipMessageKeys[skipKeyID] = skipKey
		
		newChainKey, err := dr.kdf(receivingChain.chainKey, []byte{0x02})
		if err != nil {
			return nil, fmt.Errorf("failed to update chain key: %w", err)
		}
		
		receivingChain.chainKey = newChainKey
		receivingChain.counter++
	}
	
	// Generate message keys for current message
	messageKey, nonce, err := dr.generateMessageKeys(receivingChain.chainKey, message.Counter)
	if err != nil {
		return nil, fmt.Errorf("failed to generate message keys: %w", err)
	}
	
	// Update receiving chain key
	newChainKey, err := dr.kdf(receivingChain.chainKey, []byte{0x02})
	if err != nil {
		return nil, fmt.Errorf("failed to update chain key: %w", err)
	}
	receivingChain.chainKey = newChainKey
	receivingChain.counter++
	
	// Decrypt message
	plaintext, err := dr.decryptWithAEAD(messageKey, nonce, message.Ciphertext, message.AuthTag)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt message: %w", err)
	}
	
	return plaintext, nil
}

func (dr *DoubleRatchet) dhRatchet(rootKey []byte, dhKeyPair *ecdsa.PrivateKey, dhPublicKey *ecdsa.PublicKey) ([]byte, []byte, error) {
	// Perform DH key exchange
	sharedSecret, err := ecdh(dhKeyPair, dhPublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("DH key exchange failed: %w", err)
	}
	
	// Derive new root key and chain key using HKDF
	salt := make([]byte, keySize)
	copy(salt, rootKey)
	
	hkdf := hkdf.New(sha256.New, sharedSecret, salt, []byte(infoString))
	newKeys := make([]byte, keySize*2)
	
	if _, err := hkdf.Read(newKeys); err != nil {
		return nil, nil, fmt.Errorf("HKDF failed: %w", err)
	}
	
	newRootKey := newKeys[:keySize]
	chainKey := newKeys[keySize:]
	
	return newRootKey, chainKey, nil
}

func (dr *DoubleRatchet) generateMessageKeys(chainKey []byte, counter uint32) ([]byte, []byte, error) {
	// Derive message key and nonce from chain key
	messageKeyMaterial, err := dr.kdf(chainKey, []byte{0x01})
	if err != nil {
		return nil, nil, err
	}
	
	nonceMaterial, err := dr.kdf(chainKey, []byte{0x02})
	if err != nil {
		return nil, nil, err
	}
	
	messageKey := messageKeyMaterial[:messageKeySize]
	nonce := nonceMaterial[:nonceSize]
	
	return messageKey, nonce, nil
}

func (dr *DoubleRatchet) kdf(inputKeyMaterial []byte, info []byte) ([]byte, error) {
	h := hmac.New(sha256.New, inputKeyMaterial)
	h.Write(info)
	return h.Sum(nil), nil
}

func (dr *DoubleRatchet) encryptWithAEAD(key, nonce, plaintext []byte) ([]byte, []byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	
	ciphertext := aesgcm.Seal(nil, nonce, plaintext, nil)
	
	// Split ciphertext and auth tag
	authTagSize := aesgcm.Overhead()
	ciphertextOnly := ciphertext[:len(ciphertext)-authTagSize]
	authTag := ciphertext[len(ciphertext)-authTagSize:]
	
	return ciphertextOnly, authTag, nil
}

func (dr *DoubleRatchet) decryptWithAEAD(key, nonce, ciphertext, authTag []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	
	// Combine ciphertext and auth tag
	fullCiphertext := append(ciphertext, authTag...)
	
	plaintext, err := aesgcm.Open(nil, nonce, fullCiphertext, nil)
	if err != nil {
		return nil, err
	}
	
	return plaintext, nil
}

func (dr *DoubleRatchet) CreateKeyExchangeMessage(preKeyBundle *PreKeyBundle) (*KeyExchangeMessage, error) {
	// Generate ephemeral key pair
	ephemeralKeyPair, err := GenerateEphemeralKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate ephemeral key pair: %w", err)
	}
	
	// Create message
	message := &KeyExchangeMessage{
		Version:       1,
		MessageID:     generateMessageID(),
		DHPublicKey:   publicKeyToBytes(&ephemeralKeyPair.PublicKey),
		IdentityKey:   publicKeyToBytes(&dr.identityKeyPair.PublicKey),
		SignedPreKey:  publicKeyToBytes(preKeyBundle.SignedPreKey),
	}
	
	// Sign the message
	signature, err := dr.signKeyExchangeMessage(message)
	if err != nil {
		return nil, fmt.Errorf("failed to sign key exchange message: %w", err)
	}
	message.Signature = signature
	
	return message, nil
}

func (dr *DoubleRatchet) ProcessKeyExchangeMessage(message *KeyExchangeMessage, preKeyBundle *PreKeyBundle) ([]byte, error) {
	// Verify signature
	if err := dr.verifyKeyExchangeMessage(message, preKeyBundle.IdentityKey); err != nil {
		return nil, fmt.Errorf("signature verification failed: %w", err)
	}
	
	// Parse public keys
	ephemeralPublicKey, err := parsePublicKey(message.DHPublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ephemeral public key: %w", err)
	}
	
	identityPublicKey, err := parsePublicKey(message.IdentityKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse identity public key: %w", err)
	}
	
	// Perform X3DH key agreement
	sharedSecret, err := dr.performX3DH(preKeyBundle, ephemeralPublicKey, identityPublicKey)
	if err != nil {
		return nil, fmt.Errorf("X3DH failed: %w", err)
	}
	
	// Initialize double ratchet
	rootKey := deriveRootKey(sharedSecret)
	
	if err := dr.InitializeAsBob(rootKey, ephemeralPublicKey); err != nil {
		return nil, fmt.Errorf("failed to initialize as Bob: %w", err)
	}
	
	return rootKey, nil
}

func (dr *DoubleRatchet) performX3DH(preKeyBundle *PreKeyBundle, ephemeralPublicKey, identityPublicKey *ecdsa.PublicKey) ([]byte, error) {
	// X3DH protocol implementation
	// This is a simplified version - production implementation should follow the full X3DH spec
	
	// DH1: Identity key * Signed pre key
	dh1, err := ecdh(dr.identityKeyPair, preKeyBundle.SignedPreKey)
	if err != nil {
		return nil, err
	}
	
	// DH2: Ephemeral key * Identity key
	dh2, err := ecdh(dr.identityKeyPair, identityPublicKey)
	if err != nil {
		return nil, err
	}
	
	// DH3: Ephemeral key * Signed pre key
	dh3, err := ecdh(dr.identityKeyPair, ephemeralPublicKey)
	if err != nil {
		return nil, err
	}
	
	// DH4: Ephemeral key * Pre key (if available)
	var dh4 []byte
	if preKeyBundle.PreKey != nil {
		dh4, err = ecdh(dr.identityKeyPair, preKeyBundle.PreKey)
		if err != nil {
			return nil, err
		}
	}
	
	// Combine shared secrets
	sharedSecret := append(dh1, dh2...)
	sharedSecret = append(sharedSecret, dh3...)
	if dh4 != nil {
		sharedSecret = append(sharedSecret, dh4...)
	}
	
	return sharedSecret, nil
}

func (dr *DoubleRatchet) signKeyExchangeMessage(message *KeyExchangeMessage) ([]byte, error) {
	// Create message to sign
	messageBytes := append(message.DHPublicKey, message.IdentityKey...)
	messageBytes = append(messageBytes, message.SignedPreKey...)
	
	// Sign with identity key
	hash := sha256.Sum256(messageBytes)
	signature, err := ecdsa.SignASN1(rand.Reader, dr.identityKeyPair, hash[:])
	if err != nil {
		return nil, err
	}
	
	return signature, nil
}

func (dr *DoubleRatchet) verifyKeyExchangeMessage(message *KeyExchangeMessage, identityPublicKey *ecdsa.PublicKey) error {
	// Create message to verify
	messageBytes := append(message.DHPublicKey, message.IdentityKey...)
	messageBytes = append(messageBytes, message.SignedPreKey...)
	
	// Verify signature
	hash := sha256.Sum256(messageBytes)
	return ecdsa.VerifyASN1(identityPublicKey, hash[:], message.Signature)
}

func deriveRootKey(sharedSecret []byte) []byte {
	// Derive root key from shared secret using HKDF
	salt := make([]byte, keySize)
	hkdf := hkdf.New(sha256.New, sharedSecret, salt, []byte("root_key"))
	
	rootKey := make([]byte, keySize)
	if _, err := hkdf.Read(rootKey); err != nil {
		return nil
	}
	
	return rootKey
}

func ecdh(privateKey *ecdsa.PrivateKey, publicKey *ecdsa.PublicKey) ([]byte, error) {
	x, _ := publicKey.Curve.ScalarMult(publicKey.X, publicKey.Y, privateKey.D.Bytes())
	if x == nil {
		return nil, errors.New("failed to compute shared secret")
	}
	
	sharedSecret := make([]byte, 32)
	copy(sharedSecret, x.Bytes())
	
	return sharedSecret, nil
}

func publicKeyToBytes(publicKey *ecdsa.PublicKey) []byte {
	return elliptic.Marshal(publicKey.Curve, publicKey.X, publicKey.Y)
}

func parsePublicKey(bytes []byte) (*ecdsa.PublicKey, error) {
	x, y := elliptic.Unmarshal(elliptic.P256(), bytes)
	if x == nil || y == nil {
		return nil, errors.New("failed to parse public key")
	}
	
	return &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     x,
		Y:     y,
	}, nil
}

func generateMessageID() string {
	timestamp := time.Now().UnixNano()
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	
	return fmt.Sprintf("%x-%x", timestamp, randomBytes)
}

func (dr *DoubleRatchet) GetSessionState() map[string]interface{} {
	return map[string]interface{}{
		"has_sending_chain":    dr.sendingChainKey != nil,
		"sending_counter":      dr.sendingCounter,
		"receiving_chains":     len(dr.receivingChains),
		"skip_message_keys":    len(dr.skipMessageKeys),
		"max_skip_messages":    dr.maxSkipMessages,
	}
}

func (dr *DoubleRatchet) Reset() {
	dr.rootKey = nil
	dr.dhKeyPair = nil
	dr.dhPublicKey = nil
	dr.sendingChainKey = nil
	dr.sendingCounter = 0
	dr.receivingChains = make(map[string]*ReceivingChain)
	dr.skipMessageKeys = make(map[string][]byte)
}
