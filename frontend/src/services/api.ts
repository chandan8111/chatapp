import axios from 'axios';

const API_BASE_URL = process.env.REACT_APP_API_URL || 'http://localhost:8081/api/v1';

const api = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
  timeout: 10000,
});

// Request interceptor to add auth token
api.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('token');
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => {
    return Promise.reject(error);
  }
);

// Response interceptor to handle errors
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token');
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);

// Auth API
export const authAPI = {
  login: (credentials: { email: string; password: string }) =>
    api.post('/auth/login', credentials),
  register: (userData: { username: string; email: string; password: string }) =>
    api.post('/auth/register', userData),
  logout: () => api.post('/auth/logout'),
  me: () => api.get('/auth/me'),
};

// Conversations API
export const conversationsAPI = {
  getAll: () => api.get('/conversations'),
  getById: (id: string) => api.get(`/conversations/${id}`),
  create: (data: { name: string; participants: string[]; isGroup?: boolean }) =>
    api.post('/conversations', data),
  update: (id: string, data: Partial<{ name: string; avatar?: string }>) =>
    api.put(`/conversations/${id}`, data),
  delete: (id: string) => api.delete(`/conversations/${id}`),
  addParticipant: (conversationId: string, userId: string) =>
    api.post(`/conversations/${conversationId}/participants`, { userId }),
  removeParticipant: (conversationId: string, userId: string) =>
    api.delete(`/conversations/${conversationId}/participants/${userId}`),
};

// Messages API
export const messagesAPI = {
  getByConversation: (conversationId: string, params?: { limit?: number; offset?: number; before?: number }) =>
    api.get(`/conversations/${conversationId}/messages`, { params }),
  send: (conversationId: string, content: string, type: 'text' | 'image' | 'file' = 'text') =>
    api.post(`/conversations/${conversationId}/messages`, { content, type }),
  updateStatus: (messageId: string, status: 'delivered' | 'read') =>
    api.put(`/messages/${messageId}/status`, { status }),
  delete: (messageId: string) => api.delete(`/messages/${messageId}`),
};

// Users API
export const usersAPI = {
  getAll: () => api.get('/users'),
  getById: (id: string) => api.get(`/users/${id}`),
  update: (id: string, data: Partial<{ username: string; email: string; avatar?: string; status?: string }>) =>
    api.put(`/users/${id}`, data),
  search: (query: string) => api.get('/users/search', { params: { q: query } }),
};

// Presence API
export const presenceAPI = {
  getUserPresence: (userId: string) => api.get(`/presence/${userId}`),
  getBatchPresence: (userIds: string[]) => api.post('/presence/batch', { userIds }),
  getOnlineUsers: () => api.get('/presence/online'),
  updateStatus: (status: 'online' | 'offline' | 'away') => api.put('/presence/status', { status }),
};

// Analytics API
export const analyticsAPI = {
  getMetrics: () => api.get('/analytics/metrics'),
  getHealth: () => api.get('/analytics/health'),
  getPerformance: () => api.get('/analytics/performance'),
};

export default api;
