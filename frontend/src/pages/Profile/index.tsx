import React from 'react';
import {
  Container,
  Paper,
  Typography,
  Box,
  Avatar,
  Button,
  Grid,
  TextField
} from '@mui/material';
import { useSelector } from 'react-redux';
import { RootState } from '../../store';

const Profile: React.FC = () => {
  const { user } = useSelector((state: RootState) => state.auth);

  if (!user) {
    return null;
  }

  return (
    <Container maxWidth="md">
      <Box sx={{ mt: 4, mb: 4 }}>
        <Paper elevation={3} sx={{ p: 3 }}>
          <Typography variant="h4" gutterBottom>
            Profile
          </Typography>
          
          <Grid container spacing={3}>
            <Grid item xs={12} md={4} sx={{ textAlign: 'center' }}>
              <Avatar
                sx={{ width: 120, height: 120, mx: 'auto', mb: 2 }}
                src={user.avatar}
              >
                {user.username.charAt(0).toUpperCase()}
              </Avatar>
              <Button variant="outlined" fullWidth>
                Change Avatar
              </Button>
            </Grid>
            
            <Grid item xs={12} md={8}>
              <Box component="form" sx={{ mt: 2 }}>
                <Grid container spacing={2}>
                  <Grid item xs={12} sm={6}>
                    <TextField
                      fullWidth
                      label="Username"
                      defaultValue={user.username}
                      margin="normal"
                    />
                  </Grid>
                  <Grid item xs={12} sm={6}>
                    <TextField
                      fullWidth
                      label="Email"
                      defaultValue={user.email}
                      margin="normal"
                      type="email"
                    />
                  </Grid>
                  <Grid item xs={12}>
                    <TextField
                      fullWidth
                      label="Status"
                      defaultValue={user.status || 'Hey there! I am using ChatApp'}
                      margin="normal"
                      multiline
                      rows={3}
                    />
                  </Grid>
                  <Grid item xs={12}>
                    <Box sx={{ mt: 2, display: 'flex', gap: 2 }}>
                      <Button variant="contained" color="primary">
                        Save Changes
                      </Button>
                      <Button variant="outlined" color="secondary">
                        Cancel
                      </Button>
                    </Box>
                  </Grid>
                </Grid>
              </Box>
            </Grid>
          </Grid>
        </Paper>
      </Box>
    </Container>
  );
};

export default Profile;
