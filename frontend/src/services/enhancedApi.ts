// @ts-nocheck
import axios, { AxiosInstance, AxiosRequestConfig, AxiosResponse } from 'axios';

// Enhanced API configuration
const API_CONFIG = {
  baseURL: process.env.REACT_APP_API_URL || 'http://localhost:8081/api/v1',
  timeout: 10000,
  maxRetries: 3,
  retryDelay: 1000,
  retryBackoffMultiplier: 1.5,
  maxRetryDelay: 10000,
  circuitBreakerThreshold: 5,
  circuitBreakerTimeout: 60000,
};

// Circuit breaker state
interface CircuitBreakerState {
  failures: number;
  lastFailureTime: number;
  state: 'closed' | 'open' | 'half-open';
}

// Request metrics
interface RequestMetrics {
  totalRequests: number;
  successfulRequests: number;
  failedRequests: number;
  averageResponseTime: number;
  circuitBreakerTrips: number;
}

// Enhanced error types
export class APIError extends Error {
  constructor(
    message: string,
    public status?: number,
    public code?: string,
    public retryable: boolean = false
  ) {
    super(message);
    this.name = 'APIError';
  }
}

export class RateLimitError extends APIError {
  constructor(message: string, public retryAfter: number) {
    super(message, 429, 'RATE_LIMITED', true);
    this.name = 'RateLimitError';
  }
}

export class NetworkError extends APIError {
  constructor(message: string) {
    super(message, undefined, 'NETWORK_ERROR', true);
    this.name = 'NetworkError';
  }
}

// Enhanced API client with circuit breaker and retry logic
class EnhancedAPIClient {
  private axiosInstance: AxiosInstance;
  private circuitBreaker: Map<string, CircuitBreakerState> = new Map();
  private metrics: RequestMetrics = {
    totalRequests: 0,
    successfulRequests: 0,
    failedRequests: 0,
    averageResponseTime: 0,
    circuitBreakerTrips: 0,
  };
  private responseTimes: number[] = [];

  constructor() {
    this.axiosInstance = axios.create({
      baseURL: API_CONFIG.baseURL,
      timeout: API_CONFIG.timeout,
      headers: {
        'Content-Type': 'application/json',
      },
    });

    this.setupInterceptors();
  }

  private setupInterceptors() {
    // Request interceptor
    this.axiosInstance.interceptors.request.use(
      (config) => {
        // Add auth token
        const token = localStorage.getItem('token');
        if (token) {
          config.headers.Authorization = `Bearer ${token}`;
        }

        // Add request ID for tracing
        config.headers['X-Request-ID'] = this.generateRequestId();
        
        // Add client info
        config.headers['X-Client-Version'] = process.env.REACT_APP_VERSION || '2.0.0';
        config.headers['X-Client-Platform'] = 'web';

        // Add timestamp
        config.metadata = { startTime: Date.now() };

        return config;
      },
      (error) => {
        return Promise.reject(new NetworkError(`Request setup failed: ${error.message}`));
      }
    );

    // Response interceptor
    this.axiosInstance.interceptors.response.use(
      (response: AxiosResponse) => {
        this.recordMetrics(response.config, true);
        return response;
      },
      async (error) => {
        this.recordMetrics(error.config, false);
        
        // Handle different error types
        if (error.response) {
          // Server responded with error status
          const { status, data } = error.response;
          
          if (status === 401) {
            // Unauthorized - clear token and redirect to login
            localStorage.removeItem('token');
            window.location.href = '/login';
            throw new APIError('Authentication required', status, 'UNAUTHORIZED');
          }
          
          if (status === 429) {
            // Rate limited
            const retryAfter = parseInt(error.response.headers['retry-after'] || '60');
            throw new RateLimitError('Rate limit exceeded', retryAfter);
          }
          
          if (status >= 500) {
            // Server error - retryable
            throw new APIError(data?.message || 'Server error', status, 'SERVER_ERROR', true);
          }
          
          // Client error - not retryable
          throw new APIError(data?.message || 'Request failed', status, 'CLIENT_ERROR');
        } else if (error.request) {
          // Network error - retryable
          throw new NetworkError('Network connection failed');
        } else {
          // Other error
          throw new APIError(`Request failed: ${error.message}`, undefined, 'UNKNOWN_ERROR');
        }
      }
    );
  }

  private generateRequestId(): string {
    return `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
  }

  private recordMetrics(config: AxiosRequestConfig, success: boolean) {
    this.metrics.totalRequests++;
    
    if (success) {
      this.metrics.successfulRequests++;
    } else {
      this.metrics.failedRequests++;
    }

    // Record response time
    if (config.metadata?.startTime) {
      const responseTime = Date.now() - config.metadata.startTime;
      this.responseTimes.push(responseTime);
      
      // Keep only last 100 response times
      if (this.responseTimes.length > 100) {
        this.responseTimes.shift();
      }
      
      // Calculate average
      this.metrics.averageResponseTime = Math.round(
        this.responseTimes.reduce((a, b) => a + b, 0) / this.responseTimes.length
      );
    }
  }

  private getCircuitBreakerState(endpoint: string): CircuitBreakerState {
    if (!this.circuitBreaker.has(endpoint)) {
      this.circuitBreaker.set(endpoint, {
        failures: 0,
        lastFailureTime: 0,
        state: 'closed',
      });
    }
    return this.circuitBreaker.get(endpoint)!;
  }

  private shouldAllowRequest(endpoint: string): boolean {
    const state = this.getCircuitBreakerState(endpoint);
    const now = Date.now();

    switch (state.state) {
      case 'closed':
        return true;
      
      case 'open':
        // Check if timeout has passed
        if (now - state.lastFailureTime > API_CONFIG.circuitBreakerTimeout) {
          state.state = 'half-open';
          return true;
        }
        return false;
      
      case 'half-open':
        return true;
      
      default:
        return false;
    }
  }

  private recordFailure(endpoint: string) {
    const state = this.getCircuitBreakerState(endpoint);
    state.failures++;
    state.lastFailureTime = Date.now();

    if (state.failures >= API_CONFIG.circuitBreakerThreshold) {
      if (state.state !== 'open') {
        state.state = 'open';
        this.metrics.circuitBreakerTrips++;
        console.warn(`Circuit breaker opened for endpoint: ${endpoint}`);
      }
    }
  }

  private recordSuccess(endpoint: string) {
    const state = this.getCircuitBreakerState(endpoint);
    
    if (state.state === 'half-open') {
      state.state = 'closed';
      state.failures = 0;
      console.info(`Circuit breaker closed for endpoint: ${endpoint}`);
    } else if (state.state === 'closed') {
      state.failures = Math.max(0, state.failures - 1);
    }
  }

  private async sleep(ms: number): Promise<void> {
    return new Promise(resolve => setTimeout(resolve, ms));
  }

  private async retryRequest<T>(
    fn: () => Promise<T>,
    endpoint: string,
    retryCount: number = 0
  ): Promise<T> {
    try {
      const result = await fn();
      this.recordSuccess(endpoint);
      return result;
    } catch (error) {
      this.recordFailure(endpoint);

      // Don't retry if error is not retryable or we've exceeded max retries
      if (
        retryCount >= API_CONFIG.maxRetries ||
        (error instanceof APIError && !error.retryable)
      ) {
        throw error;
      }

      // Calculate delay with exponential backoff
      const delay = Math.min(
        API_CONFIG.retryDelay * Math.pow(API_CONFIG.retryBackoffMultiplier, retryCount),
        API_CONFIG.maxRetryDelay
      );

      console.warn(`Retrying request to ${endpoint} in ${delay}ms (attempt ${retryCount + 1}/${API_CONFIG.maxRetries})`);
      
      await this.sleep(delay);
      return this.retryRequest(fn, endpoint, retryCount + 1);
    }
  }

  async request<T>(config: AxiosRequestConfig): Promise<AxiosResponse<T>> {
    const endpoint = config.url || '';
    
    // Check circuit breaker
    if (!this.shouldAllowRequest(endpoint)) {
      throw new APIError('Circuit breaker is open', 503, 'CIRCUIT_BREAKER_OPEN');
    }

    return this.retryRequest(() => this.axiosInstance.request<T>(config), endpoint);
  }

  // Convenience methods
  async get<T>(url: string, config?: AxiosRequestConfig): Promise<AxiosResponse<T>> {
    return this.request<T>({ ...config, method: 'GET', url });
  }

  async post<T>(url: string, data?: any, config?: AxiosRequestConfig): Promise<AxiosResponse<T>> {
    return this.request<T>({ ...config, method: 'POST', url, data });
  }

  async put<T>(url: string, data?: any, config?: AxiosRequestConfig): Promise<AxiosResponse<T>> {
    return this.request<T>({ ...config, method: 'PUT', url, data });
  }

  async patch<T>(url: string, data?: any, config?: AxiosRequestConfig): Promise<AxiosResponse<T>> {
    return this.request<T>({ ...config, method: 'PATCH', url, data });
  }

  async delete<T>(url: string, config?: AxiosRequestConfig): Promise<AxiosResponse<T>> {
    return this.request<T>({ ...config, method: 'DELETE', url });
  }

  // Get metrics
  getMetrics(): RequestMetrics {
    return { ...this.metrics };
  }

  // Get circuit breaker status
  getCircuitBreakerStatus(): Record<string, CircuitBreakerState> {
    const status: Record<string, CircuitBreakerState> = {};
    this.circuitBreaker.forEach((state, endpoint) => {
      status[endpoint] = { ...state };
    });
    return status;
  }

  // Reset circuit breaker for endpoint
  resetCircuitBreaker(endpoint: string) {
    const state = this.getCircuitBreakerState(endpoint);
    state.failures = 0;
    state.state = 'closed';
    state.lastFailureTime = 0;
    console.info(`Circuit breaker reset for endpoint: ${endpoint}`);
  }

  // Reset all circuit breakers
  resetAllCircuitBreakers() {
    this.circuitBreaker.forEach((state, endpoint) => {
      this.resetCircuitBreaker(endpoint);
    });
  }
}

// Create singleton instance
const apiClient = new EnhancedAPIClient();

// Enhanced API services with better error handling
export const enhancedAuthAPI = {
  login: async (credentials: { email: string; password: string }) => {
    try {
      const response = await apiClient.post('/auth/login', credentials);
      return response;
    } catch (error) {
      if (error instanceof APIError) {
        throw error;
      }
      throw new APIError('Login failed. Please try again.');
    }
  },

  register: async (userData: { username: string; email: string; password: string }) => {
    try {
      const response = await apiClient.post('/auth/register', userData);
      return response;
    } catch (error) {
      if (error instanceof APIError) {
        throw error;
      }
      throw new APIError('Registration failed. Please try again.');
    }
  },

  logout: async () => {
    try {
      const response = await apiClient.post('/auth/logout');
      localStorage.removeItem('token');
      return response;
    } catch (error) {
      // Always remove token locally even if request fails
      localStorage.removeItem('token');
      if (error instanceof APIError) {
        throw error;
      }
      throw new APIError('Logout completed locally.');
    }
  },

  me: async () => {
    try {
      const response = await apiClient.get('/auth/me');
      return response;
    } catch (error) {
      if (error instanceof APIError) {
        throw error;
      }
      throw new APIError('Failed to fetch user information.');
    }
  },

  refreshToken: async () => {
    try {
      const response = await apiClient.post('/auth/refresh');
      if (response.data.token) {
        localStorage.setItem('token', response.data.token);
      }
      return response;
    } catch (error) {
      if (error instanceof APIError) {
        throw error;
      }
      throw new APIError('Failed to refresh token.');
    }
  },
};

export const enhancedConversationsAPI = {
  getAll: async () => {
    try {
      const response = await apiClient.get('/conversations');
      return response;
    } catch (error) {
      if (error instanceof APIError) {
        throw error;
      }
      throw new APIError('Failed to load conversations.');
    }
  },

  getById: async (id: string) => {
    try {
      const response = await apiClient.get(`/conversations/${id}`);
      return response;
    } catch (error) {
      if (error instanceof APIError) {
        throw error;
      }
      throw new APIError('Failed to load conversation.');
    }
  },

  create: async (data: { name: string; participants: string[]; isGroup: boolean }) => {
    try {
      const response = await apiClient.post('/conversations', data);
      return response;
    } catch (error) {
      if (error instanceof APIError) {
        throw error;
      }
      throw new APIError('Failed to create conversation.');
    }
  },

  update: async (id: string, data: Partial<{ name: string; avatar: string }>) => {
    try {
      const response = await apiClient.patch(`/conversations/${id}`, data);
      return response;
    } catch (error) {
      if (error instanceof APIError) {
        throw error;
      }
      throw new APIError('Failed to update conversation.');
    }
  },

  leave: async (id: string) => {
    try {
      const response = await apiClient.post(`/conversations/${id}/leave`);
      return response;
    } catch (error) {
      if (error instanceof APIError) {
        throw error;
      }
      throw new APIError('Failed to leave conversation.');
    }
  },
};

export const enhancedMessagesAPI = {
  getByConversation: async (conversationId: string, params?: { limit?: number; offset?: string }) => {
    try {
      const response = await apiClient.get(`/conversations/${conversationId}/messages`, { params });
      return response;
    } catch (error) {
      if (error instanceof APIError) {
        throw error;
      }
      throw new APIError('Failed to load messages.');
    }
  },

  send: async (conversationId: string, data: { content: string; type: string; metadata?: any }) => {
    try {
      const response = await apiClient.post(`/conversations/${conversationId}/messages`, data);
      return response;
    } catch (error) {
      if (error instanceof APIError) {
        throw error;
      }
      throw new APIError('Failed to send message.');
    }
  },

  markAsRead: async (conversationId: string, messageId: string) => {
    try {
      const response = await apiClient.post(`/conversations/${conversationId}/messages/${messageId}/read`);
      return response;
    } catch (error) {
      if (error instanceof APIError) {
        throw error;
      }
      throw new APIError('Failed to mark message as read.');
    }
  },

  delete: async (conversationId: string, messageId: string) => {
    try {
      const response = await apiClient.delete(`/conversations/${conversationId}/messages/${messageId}`);
      return response;
    } catch (error) {
      if (error instanceof APIError) {
        throw error;
      }
      throw new APIError('Failed to delete message.');
    }
  },
};

export const enhancedUsersAPI = {
  search: async (query: string, params?: { limit?: number }) => {
    try {
      const response = await apiClient.get('/users/search', { 
        params: { q: query, ...params } 
      });
      return response;
    } catch (error) {
      if (error instanceof APIError) {
        throw error;
      }
      throw new APIError('Failed to search users.');
    }
  },

  getProfile: async (userId: string) => {
    try {
      const response = await apiClient.get(`/users/${userId}`);
      return response;
    } catch (error) {
      if (error instanceof APIError) {
        throw error;
      }
      throw new APIError('Failed to load user profile.');
    }
  },

  updateProfile: async (data: Partial<{ username: string; avatar: string; status: string }>) => {
    try {
      const response = await apiClient.patch('/users/profile', data);
      return response;
    } catch (error) {
      if (error instanceof APIError) {
        throw error;
      }
      throw new APIError('Failed to update profile.');
    }
  },
};

// Export the client for advanced usage
export { apiClient };

// Export utilities
export const getAPIMetrics = () => apiClient.getMetrics();
export const getCircuitBreakerStatus = () => apiClient.getCircuitBreakerStatus();
export const resetCircuitBreaker = (endpoint: string) => apiClient.resetCircuitBreaker(endpoint);
export const resetAllCircuitBreakers = () => apiClient.resetAllCircuitBreakers();
