# ChatApp Android Application Design

## Overview

The ChatApp Android application is a native mobile client for the ChatApp distributed chat system. Built with modern Android development practices, it provides real-time messaging, end-to-end encryption, and seamless integration with the backend services.

## Architecture

### Technology Stack
- **Language**: Kotlin
- **UI Framework**: Jetpack Compose
- **Architecture**: MVVM + Clean Architecture
- **Dependency Injection**: Hilt
- **Networking**: Retrofit + OkHttp + WebSocket (Scarlet)
- **Database**: Room (SQLite)
- **Reactive Programming**: Kotlin Coroutines + Flow
- **Security**: Android Keystore + LibSignal Protocol
- **Push Notifications**: Firebase Cloud Messaging
- **Offline Support**: WorkManager + SyncAdapter

### Architecture Diagram
```
┌─────────────────────────────────────────────────────────────┐
│                    Presentation Layer                        │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   Screens    │  │  ViewModels  │  │    States    │      │
│  │  (Compose)   │  │              │  │              │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                     Domain Layer                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   Use Cases  │  │   Entities   │  │  Repository  │      │
│  │              │  │              │  │  Interfaces  │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                     Data Layer                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ Repositories │  │   Remote     │  │    Local     │      │
│  │              │  │   (API/WS)   │  │  (Room DB)   │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└─────────────────────────────────────────────────────────────┘
```

## Core Features

### 1. Real-time Messaging
- WebSocket connection for instant message delivery
- Offline message queue with automatic sync
- Message status tracking (sent, delivered, read)
- Typing indicators
- Read receipts

### 2. End-to-End Encryption
- Signal Protocol (Double Ratchet) implementation
- Secure key storage in Android Keystore
- Message encryption/decryption
- Forward secrecy
- Group encryption support

### 3. User Interface
- Modern Material Design 3
- Jetpack Compose for declarative UI
- Dark/Light theme support
- Responsive layouts for all screen sizes
- Smooth animations and transitions

### 4. Offline Support
- Full offline functionality
- Message drafting
- Media caching
- Background sync
- Conflict resolution

### 5. Notifications
- Push notifications via FCM
- Rich notifications with actions
- Notification channels
- Quiet hours support

## Project Structure

```
app/
├── src/
│   ├── main/
│   │   ├── java/com/chatapp/
│   │   │   ├── ChatApp.kt              # Application class
│   │   │   ├── di/                     # Dependency Injection
│   │   │   │   ├── AppModule.kt
│   │   │   │   ├── NetworkModule.kt
│   │   │   │   └── DatabaseModule.kt
│   │   │   ├── data/                   # Data Layer
│   │   │   │   ├── local/
│   │   │   │   │   ├── dao/
│   │   │   │   │   ├── entity/
│   │   │   │   │   └── ChatDatabase.kt
│   │   │   │   ├── remote/
│   │   │   │   │   ├── api/
│   │   │   │   │   ├── websocket/
│   │   │   │   │   └── dto/
│   │   │   │   └── repository/
│   │   │   ├── domain/                 # Domain Layer
│   │   │   │   ├── model/
│   │   │   │   ├── repository/
│   │   │   │   └── usecase/
│   │   │   ├── presentation/           # Presentation Layer
│   │   │   │   ├── components/
│   │   │   │   ├── screens/
│   │   │   │   ├── viewmodel/
│   │   │   │   └── state/
│   │   │   ├── security/               # Security Layer
│   │   │   │   ├── encryption/
│   │   │   │   └── keystore/
│   │   │   └── utils/
│   │   └── res/                        # Resources
│   └── test/                           # Unit & Integration Tests
├── build.gradle.kts
└── AndroidManifest.xml
```

## Key Components

### 1. WebSocket Manager
```kotlin
@Singleton
class WebSocketManager @Inject constructor(
    private val scarlet: Scarlet,
    private val messageDao: MessageDao,
    @ApplicationContext private val context: Context
) {
    private lateinit var webSocket: WebSocket
    private val _connectionState = MutableStateFlow(ConnectionState.DISCONNECTED)
    val connectionState: StateFlow<ConnectionState> = _connectionState.asStateFlow()

    fun connect(token: String) {
        webSocket = scarlet.create()
        webSocket.observeWebSocketEvent()
            .flowOn(Dispatchers.IO)
            .onEach { event ->
                when (event) {
                    is WebSocketEvent.OnConnectionOpened -> {
                        _connectionState.value = ConnectionState.CONNECTED
                        authenticate(token)
                    }
                    is WebSocketEvent.OnMessageReceived -> handleMessage(event)
                    is WebSocketEvent.OnConnectionClosed -> {
                        _connectionState.value = ConnectionState.DISCONNECTED
                        reconnect()
                    }
                }
            }.launchIn(scope)
    }
}
```

### 2. Encryption Service
```kotlin
@Singleton
class EncryptionService @Inject constructor(
    private val keyStoreManager: KeyStoreManager,
    private val signalProtocol: SignalProtocol
) {
    suspend fun encryptMessage(
        recipientId: String,
        plaintext: String
    ): EncryptedMessage {
        val sessionCipher = getSessionCipher(recipientId)
        val ciphertext = sessionCipher.encrypt(plaintext.toByteArray())
        return EncryptedMessage(
            recipientId = recipientId,
            ciphertext = ciphertext.serialize(),
            timestamp = System.currentTimeMillis()
        )
    }

    suspend fun decryptMessage(
        senderId: String,
        ciphertext: ByteArray
    ): String {
        val sessionCipher = getSessionCipher(senderId)
        val plaintext = sessionCipher.decrypt(PreKeySignalMessage(ciphertext))
        return String(plaintext)
    }
}
```

### 3. Sync Manager
```kotlin
@HiltAndroidApp
class SyncManager @Inject constructor(
    private val workManager: WorkManager,
    private val messageRepository: MessageRepository
) {
    fun scheduleSync() {
        val constraints = Constraints.Builder()
            .setRequiredNetworkType(NetworkType.CONNECTED)
            .build()

        val syncWork = PeriodicWorkRequestBuilder<SyncWorker>(15, TimeUnit.MINUTES)
            .setConstraints(constraints)
            .build()

        workManager.enqueueUniquePeriodicWork(
            "message_sync",
            ExistingPeriodicWorkPolicy.KEEP,
            syncWork
        )
    }
}
```

## UI Screens

### 1. Authentication Screens
- **Splash Screen**: App logo and loading
- **Login Screen**: Email/password authentication
- **Registration Screen**: New user registration
- **Verification Screen**: 2FA/Email verification

### 2. Main Screens
- **Conversation List**: All chats with preview
- **Chat Screen**: Individual conversation view
- **Contacts**: User search and management
- **Settings**: App configuration
- **Profile**: User profile management

### 3. Compose Components
```kotlin
@Composable
fun ChatScreen(
    viewModel: ChatViewModel = hiltViewModel(),
    conversationId: String
) {
    val uiState by viewModel.uiState.collectAsState()
    val messages = uiState.messages
    val currentUser = uiState.currentUser

    Scaffold(
        topBar = { ChatTopBar(uiState.conversation) },
        bottomBar = { MessageInputBar(onSend = viewModel::sendMessage) }
    ) { padding ->
        LazyColumn(
            modifier = Modifier.padding(padding),
            reverseLayout = true
        ) {
            items(messages) { message ->
                MessageBubble(
                    message = message,
                    isCurrentUser = message.senderId == currentUser.id
                )
            }
        }
    }
}
```

## Database Schema

### Room Entities
```kotlin
@Entity(tableName = "messages")
data class MessageEntity(
    @PrimaryKey val id: String,
    val conversationId: String,
    val senderId: String,
    val content: String,
    val encryptedContent: ByteArray?,
    val timestamp: Long,
    val status: MessageStatus,
    val type: MessageType,
    val synced: Boolean = false
)

@Entity(tableName = "conversations")
data class ConversationEntity(
    @PrimaryKey val id: String,
    val name: String,
    val avatarUrl: String?,
    val isGroup: Boolean,
    val lastMessageId: String?,
    val lastMessageTime: Long?,
    val unreadCount: Int = 0,
    val createdAt: Long
)

@Entity(tableName = "users")
data class UserEntity(
    @PrimaryKey val id: String,
    val username: String,
    val email: String,
    val avatarUrl: String?,
    val publicKey: ByteArray,
    val status: UserStatus,
    val lastSeen: Long?
)
```

## Security Implementation

### 1. Key Management
- Master key stored in Android Keystore
- Signal Protocol identity key pair
- Per-session encryption keys
- Automatic key rotation

### 2. Secure Storage
- Encrypted SharedPreferences
- SQLCipher for database encryption
- Certificate pinning for API calls
- Biometric authentication support

### 3. Network Security
- TLS 1.3 for all connections
- Certificate pinning
- Network security configuration
- ProGuard/R8 code obfuscation

## Performance Optimizations

### 1. Memory Management
- Image loading with Coil (caching)
- Lazy loading for messages
- Message pagination
- Database query optimization

### 2. Battery Optimization
- WorkManager for background tasks
- Doze mode support
- Network batching
- Efficient WebSocket usage

### 3. UI Performance
- Compose lazy lists
- Image caching
- Smooth animations
- Reduced recompositions

## Testing Strategy

### 1. Unit Tests
- ViewModel tests
- Use case tests
- Repository tests
- Encryption tests

### 2. Integration Tests
- Database tests
- API tests
- WebSocket tests
- Sync tests

### 3. UI Tests
- Compose UI tests
- End-to-end flows
- Accessibility tests

## Build Configuration

```kotlin
// build.gradle.kts (App level)
plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("com.google.devtools.ksp")
    id("com.google.dagger.hilt.android")
    id("com.google.gms.google-services")
}

android {
    namespace = "com.chatapp"
    compileSdk = 34

    defaultConfig {
        applicationId = "com.chatapp"
        minSdk = 26
        targetSdk = 34
        versionCode = 1
        versionName = "1.0.0"

        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
    }

    buildTypes {
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro"
            )
        }
    }

    buildFeatures {
        compose = true
    }

    composeOptions {
        kotlinCompilerExtensionVersion = "1.5.3"
    }

    kotlinOptions {
        jvmTarget = "1.8"
    }
}

dependencies {
    // Android Core
    implementation("androidx.core:core-ktx:1.12.0")
    implementation("androidx.lifecycle:lifecycle-runtime-ktx:2.6.2")
    implementation("androidx.activity:activity-compose:1.8.0")

    // Compose
    implementation(platform("androidx.compose:compose-bom:2023.10.01"))
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.ui:ui-graphics")
    implementation("androidx.compose.ui:ui-tooling-preview")
    implementation("androidx.compose.material3:material3")

    // Navigation
    implementation("androidx.navigation:navigation-compose:2.7.5")

    // Hilt DI
    implementation("com.google.dagger:hilt-android:2.48")
    ksp("com.google.dagger:hilt-compiler:2.48")
    implementation("androidx.hilt:hilt-navigation-compose:1.1.0")

    // Networking
    implementation("com.squareup.retrofit2:retrofit:2.9.0")
    implementation("com.squareup.retrofit2:converter-gson:2.9.0")
    implementation("com.squareup.okhttp3:logging-interceptor:4.11.0")
    implementation("com.tinder.scarlet:scarlet:0.1.12")
    implementation("com.tinder.scarlet:websocket-okhttp:0.1.12")
    implementation("com.tinder.scarlet:lifecycle-android:0.1.12")

    // Database
    implementation("androidx.room:room-runtime:2.6.0")
    implementation("androidx.room:room-ktx:2.6.0")
    ksp("androidx.room:room-compiler:2.6.0")

    // Security
    implementation("androidx.security:security-crypto:1.1.0-alpha06")
    implementation("org.signal:libsignal-client:0.32.0")

    // Firebase
    implementation(platform("com.google.firebase:firebase-bom:32.5.0"))
    implementation("com.google.firebase:firebase-messaging-ktx")
    implementation("com.google.firebase:firebase-analytics-ktx")

    // Image Loading
    implementation("io.coil-kt:coil-compose:2.5.0")

    // WorkManager
    implementation("androidx.work:work-runtime-ktx:2.9.0")
    implementation("androidx.hilt:hilt-work:1.1.0")

    // Testing
    testImplementation("junit:junit:4.13.2")
    testImplementation("org.jetbrains.kotlinx:kotlinx-coroutines-test:1.7.3")
    testImplementation("io.mockk:mockk:1.13.8")
    androidTestImplementation("androidx.test.ext:junit:1.1.5")
    androidTestImplementation("androidx.test.espresso:espresso-core:3.5.1")
    androidTestImplementation(platform("androidx.compose:compose-bom:2023.10.01"))
    androidTestImplementation("androidx.compose.ui:ui-test-junit4")
    debugImplementation("androidx.compose.ui:ui-tooling")
    debugImplementation("androidx.compose.ui:ui-test-manifest")
}
```

## Deployment

### 1. Google Play Store
- Release builds with ProGuard
- App signing with Google Play
- Internal testing track
- Staged rollouts

### 2. Firebase App Distribution
- Beta testing
- Crashlytics integration
- Performance monitoring
- A/B testing

## Future Enhancements

### Version 2.0
- Voice messages
- Video calls (WebRTC)
- Stories/Status
- Dark mode improvements
- Tablet optimization

### Version 3.0
- AI features
- Smart replies
- Message translation
- Advanced search
- Chatbots integration

---

**Built with ❤️ using Kotlin, Jetpack Compose, and modern Android architecture.**
