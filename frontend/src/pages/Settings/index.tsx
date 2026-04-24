import React from 'react';
import {
  Container,
  Paper,
  Typography,
  Box,
  Switch,
  Divider,
  Button,
  List,
  ListItem,
  ListItemText,
  ListItemSecondaryAction
} from '@mui/material';

const Settings: React.FC = () => {
  const [notifications, setNotifications] = React.useState(true);
  const [soundEnabled, setSoundEnabled] = React.useState(true);
  const [darkMode, setDarkMode] = React.useState(false);
  const [onlineStatus, setOnlineStatus] = React.useState(true);

  return (
    <Container maxWidth="md">
      <Box sx={{ mt: 4, mb: 4 }}>
        <Paper elevation={3} sx={{ p: 3 }}>
          <Typography variant="h4" gutterBottom>
            Settings
          </Typography>
          
          <List>
            <Typography variant="h6" sx={{ mt: 2, mb: 1 }}>
              Notifications
            </Typography>
            <ListItem>
              <ListItemText
                primary="Push Notifications"
                secondary="Receive notifications for new messages"
              />
              <ListItemSecondaryAction>
                <Switch
                  edge="end"
                  checked={notifications}
                  onChange={(e) => setNotifications(e.target.checked)}
                />
              </ListItemSecondaryAction>
            </ListItem>
            
            <ListItem>
              <ListItemText
                primary="Sound Effects"
                secondary="Play sound for incoming messages"
              />
              <ListItemSecondaryAction>
                <Switch
                  edge="end"
                  checked={soundEnabled}
                  onChange={(e) => setSoundEnabled(e.target.checked)}
                />
              </ListItemSecondaryAction>
            </ListItem>
            
            <Divider sx={{ my: 2 }} />
            
            <Typography variant="h6" sx={{ mt: 2, mb: 1 }}>
              Appearance
            </Typography>
            <ListItem>
              <ListItemText
                primary="Dark Mode"
                secondary="Use dark theme"
              />
              <ListItemSecondaryAction>
                <Switch
                  edge="end"
                  checked={darkMode}
                  onChange={(e) => setDarkMode(e.target.checked)}
                />
              </ListItemSecondaryAction>
            </ListItem>
            
            <Divider sx={{ my: 2 }} />
            
            <Typography variant="h6" sx={{ mt: 2, mb: 1 }}>
              Privacy
            </Typography>
            <ListItem>
              <ListItemText
                primary="Online Status"
                secondary="Show when you are online"
              />
              <ListItemSecondaryAction>
                <Switch
                  edge="end"
                  checked={onlineStatus}
                  onChange={(e) => setOnlineStatus(e.target.checked)}
                />
              </ListItemSecondaryAction>
            </ListItem>
            
            <Divider sx={{ my: 2 }} />
            
            <Typography variant="h6" sx={{ mt: 2, mb: 1 }}>
              Account
            </Typography>
            <ListItem>
              <ListItemText
                primary="Clear Cache"
                secondary="Clear local data and cache"
              />
              <ListItemSecondaryAction>
                <Button variant="outlined" size="small">
                  Clear
                </Button>
              </ListItemSecondaryAction>
            </ListItem>
            
            <ListItem>
              <ListItemText
                primary="Export Data"
                secondary="Download your chat history"
              />
              <ListItemSecondaryAction>
                <Button variant="outlined" size="small">
                  Export
                </Button>
              </ListItemSecondaryAction>
            </ListItem>
            
            <ListItem>
              <ListItemText
                primary="Delete Account"
                secondary="Permanently delete your account"
              />
              <ListItemSecondaryAction>
                <Button variant="outlined" color="error" size="small">
                  Delete
                </Button>
              </ListItemSecondaryAction>
            </ListItem>
          </List>
        </Paper>
      </Box>
    </Container>
  );
};

export default Settings;
