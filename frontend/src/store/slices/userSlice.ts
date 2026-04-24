import { createSlice, createAsyncThunk, PayloadAction } from '@reduxjs/toolkit';

interface User {
  id: string;
  username: string;
  email: string;
  avatar?: string;
  status?: string;
  isOnline?: boolean;
  lastSeen?: string;
}

interface UserState {
  users: Record<string, User>;
  loading: boolean;
  error: string | null;
}

const initialState: UserState = {
  users: {},
  loading: false,
  error: null,
};

// Async thunks
export const fetchUsers = createAsyncThunk(
  'user/fetchUsers',
  async () => {
    // Placeholder implementation
    return [];
  }
);

export const updateUserProfile = createAsyncThunk(
  'user/updateProfile',
  async (userData: Partial<User>) => {
    // Placeholder implementation
    return userData;
  }
);

const userSlice = createSlice({
  name: 'user',
  initialState,
  reducers: {
    updateUser: (state, action: PayloadAction<User>) => {
      state.users[action.payload.id] = action.payload;
    },
    removeUser: (state, action: PayloadAction<string>) => {
      delete state.users[action.payload];
    },
    clearUsers: (state) => {
      state.users = {};
    },
    setUserOnlineStatus: (state, action: PayloadAction<{ userId: string; isOnline: boolean }>) => {
      const { userId, isOnline } = action.payload;
      if (state.users[userId]) {
        state.users[userId].isOnline = isOnline;
        state.users[userId].lastSeen = new Date().toISOString();
      }
    },
  },
  extraReducers: (builder) => {
    builder
      .addCase(fetchUsers.pending, (state) => {
        state.loading = true;
        state.error = null;
      })
      .addCase(fetchUsers.fulfilled, (state, action) => {
        state.loading = false;
        const users = action.payload;
        users.forEach((user: User) => {
          state.users[user.id] = user;
        });
      })
      .addCase(fetchUsers.rejected, (state, action) => {
        state.loading = false;
        state.error = action.error.message || 'Failed to fetch users';
      })
      .addCase(updateUserProfile.pending, (state) => {
        state.loading = true;
        state.error = null;
      })
      .addCase(updateUserProfile.fulfilled, (state, action) => {
        state.loading = false;
        if (action.payload.id) {
          state.users[action.payload.id] = { ...state.users[action.payload.id], ...action.payload };
        }
      })
      .addCase(updateUserProfile.rejected, (state, action) => {
        state.loading = false;
        state.error = action.error.message || 'Failed to update profile';
      });
  },
});

export const { updateUser, removeUser, clearUsers, setUserOnlineStatus } = userSlice.actions;
export default userSlice.reducer;
