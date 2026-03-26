// @ts-nocheck
import React, { useState, useEffect } from 'react';
import {
  Box,
  Card,
  CardContent,
  Typography,
  Grid,
  LinearProgress,
  Chip,
  Alert,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Paper,
  ToggleButton,
  ToggleButtonGroup,
  Button,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  List,
  ListItem,
  ListItemText,
  Divider,
} from '@mui/material';
import {
  Refresh as RefreshIcon,
  Settings as SettingsIcon,
  Warning as WarningIcon,
  Error as ErrorIcon,
  CheckCircle as CheckCircleIcon,
  NetworkCheck as NetworkIcon,
  Memory as MemoryIcon,
  Speed as SpeedIcon,
  Timeline as TimelineIcon,
} from '@mui/icons-material';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, AreaChart, Area } from 'recharts';
import { getAPIMetrics, getCircuitBreakerStatus, resetCircuitBreaker } from '../../services/enhancedApi';

// Metrics types
interface APIMetrics {
  totalRequests: number;
  successfulRequests: number;
  failedRequests: number;
  averageResponseTime: number;
  circuitBreakerTrips: number;
}

interface CircuitBreakerStatus {
  [endpoint: string]: {
    failures: number;
    lastFailureTime: number;
    state: 'closed' | 'open' | 'half-open';
  };
}

interface WebSocketMetrics {
  connectionTime: number;
  messagesSent: number;
  messagesReceived: number;
  reconnections: number;
  errors: number;
  lastPing: number;
  averageLatency: number;
}

interface PerformanceData {
  timestamp: number;
  responseTime: number;
  successRate: number;
  requestCount: number;
  errorCount: number;
}

const MonitoringDashboard: React.FC = () => {
  const [apiMetrics, setAPIMetrics] = useState<APIMetrics | null>(null);
  const [circuitBreakerStatus, setCircuitBreakerStatus] = useState<CircuitBreakerStatus>({});
  const [webSocketMetrics, setWebSocketMetrics] = useState<WebSocketMetrics | null>(null);
  const [performanceData, setPerformanceData] = useState<PerformanceData[]>([]);
  const [selectedTimeRange, setSelectedTimeRange] = useState<string>('1h');
  const [autoRefresh, setAutoRefresh] = useState<boolean>(true);
  const [refreshInterval, setRefreshInterval] = useState<number>(5000);
  const [error, setError] = useState<string | null>(null);
  const [settingsOpen, setSettingsOpen] = useState<boolean>(false);
  const [lastRefresh, setLastRefresh] = useState<Date>(new Date());

  // Load metrics
  const loadMetrics = async () => {
    try {
      const [apiMetrics, circuitStatus] = await Promise.all([
        getAPIMetrics(),
        getCircuitBreakerStatus(),
      ]);

      setAPIMetrics(apiMetrics);
      setCircuitBreakerStatus(circuitStatus);
      setError(null);

      // Add to performance data
      const newDataPoint: PerformanceData = {
        timestamp: Date.now(),
        responseTime: apiMetrics.averageResponseTime,
        successRate: apiMetrics.totalRequests > 0 
          ? (apiMetrics.successfulRequests / apiMetrics.totalRequests) * 100 
          : 100,
        requestCount: apiMetrics.totalRequests,
        errorCount: apiMetrics.failedRequests,
      };

      setPerformanceData(prev => {
        const updated = [...prev, newDataPoint];
        // Keep only last 100 data points
        return updated.slice(-100);
      });

      setLastRefresh(new Date());
    } catch (err) {
      setError('Failed to load metrics');
      console.error('Metrics loading error:', err);
    }
  };

  // Auto-refresh effect
  useEffect(() => {
    loadMetrics();

    if (autoRefresh) {
      const interval = setInterval(loadMetrics, refreshInterval);
      return () => clearInterval(interval);
    }
  }, [autoRefresh, refreshInterval]);

  // Calculate health score
  const calculateHealthScore = () => {
    if (!apiMetrics) return 0;

    const successRate = apiMetrics.totalRequests > 0 
      ? (apiMetrics.successfulRequests / apiMetrics.totalRequests) * 100 
      : 100;
    
    const responseTimeScore = Math.max(0, 100 - (apiMetrics.averageResponseTime / 10)); // 10s = 0 score
    const errorScore = Math.max(0, 100 - (apiMetrics.failedRequests / Math.max(apiMetrics.totalRequests, 1)) * 100);

    return Math.round((successRate + responseTimeScore + errorScore) / 3);
  };

  // Get status color
  const getStatusColor = (value: number, thresholds: { good: number; warning: number }) => {
    if (value >= thresholds.good) return 'success';
    if (value >= thresholds.warning) return 'warning';
    return 'error';
  };

  // Format time
  const formatTime = (timestamp: number) => {
    return new Date(timestamp).toLocaleTimeString();
  };

  // Reset circuit breaker
  const handleResetCircuitBreaker = (endpoint: string) => {
    resetCircuitBreaker(endpoint);
    loadMetrics();
  };

  // Health score card
  const HealthScoreCard = () => {
    const healthScore = calculateHealthScore();
    const healthStatus = healthScore >= 80 ? 'healthy' : healthScore >= 60 ? 'warning' : 'critical';

    return (
      <Card>
        <CardContent>
          <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 2 }}>
            <Typography variant="h6" component="div">
              System Health
            </Typography>
            <Chip
              icon={healthStatus === 'healthy' ? <CheckCircleIcon /> : 
                    healthStatus === 'warning' ? <WarningIcon /> : <ErrorIcon />}
              label={healthStatus.toUpperCase()}
              color={healthStatus === 'healthy' ? 'success' : healthStatus === 'warning' ? 'warning' : 'error'}
              size="small"
            />
          </Box>
          
          <Box sx={{ position: 'relative', display: 'inline-flex' }}>
            <Box sx={{ position: 'relative', display: 'inline-flex' }}>
              <CircularProgress
                variant="determinate"
                value={healthScore}
                size={120}
                thickness={8}
                color={healthStatus === 'healthy' ? 'success' : healthStatus === 'warning' ? 'warning' : 'error'}
              />
              <Box
                sx={{
                  top: 0,
                  left: 0,
                  bottom: 0,
                  right: 0,
                  position: 'absolute',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                }}
              >
                <Typography variant="h4" component="div" color="text.secondary">
                  {healthScore}%
                </Typography>
              </Box>
            </Box>
          </Box>
          
          <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
            Last updated: {lastRefresh.toLocaleTimeString()}
          </Typography>
        </CardContent>
      </Card>
    );
  };

  // Metrics overview cards
  const MetricsOverview = () => (
    <Grid container spacing={3}>
      <Grid item xs={12} sm={6} md={3}>
        <Card>
          <CardContent>
            <Box sx={{ display: 'flex', alignItems: 'center', mb: 1 }}>
              <SpeedIcon color="primary" />
              <Typography variant="h6" sx={{ ml: 1 }}>
                Response Time
              </Typography>
            </Box>
            <Typography variant="h4" component="div">
              {apiMetrics?.averageResponseTime || 0}ms
            </Typography>
            <LinearProgress
              variant="determinate"
              value={Math.min(100, (apiMetrics?.averageResponseTime || 0) / 100)}
              color={getStatusColor(apiMetrics?.averageResponseTime || 0, { good: 200, warning: 500 }) as any}
              sx={{ mt: 1 }}
            />
          </CardContent>
        </Card>
      </Grid>

      <Grid item xs={12} sm={6} md={3}>
        <Card>
          <CardContent>
            <Box sx={{ display: 'flex', alignItems: 'center', mb: 1 }}>
              <TimelineIcon color="success" />
              <Typography variant="h6" sx={{ ml: 1 }}>
                Success Rate
              </Typography>
            </Box>
            <Typography variant="h4" component="div">
              {apiMetrics?.totalRequests ? 
                Math.round((apiMetrics.successfulRequests / apiMetrics.totalRequests) * 100) : 100}%
            </Typography>
            <LinearProgress
              variant="determinate"
              value={apiMetrics?.totalRequests ? 
                (apiMetrics.successfulRequests / apiMetrics.totalRequests) * 100 : 100}
              color="success"
              sx={{ mt: 1 }}
            />
          </CardContent>
        </Card>
      </Grid>

      <Grid item xs={12} sm={6} md={3}>
        <Card>
          <CardContent>
            <Box sx={{ display: 'flex', alignItems: 'center', mb: 1 }}>
              <NetworkIcon color="info" />
              <Typography variant="h6" sx={{ ml: 1 }}>
                Total Requests
              </Typography>
            </Box>
            <Typography variant="h4" component="div">
              {apiMetrics?.totalRequests || 0}
            </Typography>
            <Typography variant="body2" color="text.secondary">
              {apiMetrics?.failedRequests || 0} errors
            </Typography>
          </CardContent>
        </Card>
      </Grid>

      <Grid item xs={12} sm={6} md={3}>
        <Card>
          <CardContent>
            <Box sx={{ display: 'flex', alignItems: 'center', mb: 1 }}>
              <WarningIcon color="warning" />
              <Typography variant="h6" sx={{ ml: 1 }}>
                Circuit Breakers
              </Typography>
            </Box>
            <Typography variant="h4" component="div">
              {Object.values(circuitBreakerStatus).filter(cb => cb.state === 'open').length}
            </Typography>
            <Typography variant="body2" color="text.secondary">
              {Object.keys(circuitBreakerStatus).length} total
            </Typography>
          </CardContent>
        </Card>
      </Grid>
    </Grid>
  );

  // Performance charts
  const PerformanceCharts = () => (
    <Grid container spacing={3}>
      <Grid item xs={12} md={8}>
        <Card>
          <CardContent>
            <Typography variant="h6" gutterBottom>
              Response Time Trend
            </Typography>
            <ResponsiveContainer width="100%" height={300}>
              <LineChart data={performanceData}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis 
                  dataKey="timestamp"
                  tickFormatter={formatTime}
                />
                <YAxis />
                <Tooltip 
                  labelFormatter={formatTime}
                  formatter={(value: any) => [`${value}ms`, 'Response Time']}
                />
                <Line 
                  type="monotone" 
                  dataKey="responseTime" 
                  stroke="#8884d8" 
                  strokeWidth={2}
                  dot={false}
                />
              </LineChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>
      </Grid>

      <Grid item xs={12} md={4}>
        <Card>
          <CardContent>
            <Typography variant="h6" gutterBottom>
              Success Rate
            </Typography>
            <ResponsiveContainer width="100%" height={300}>
              <AreaChart data={performanceData}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis 
                  dataKey="timestamp"
                  tickFormatter={formatTime}
                />
                <YAxis domain={[0, 100]} />
                <Tooltip 
                  labelFormatter={formatTime}
                  formatter={(value: any) => [`${value}%`, 'Success Rate']}
                />
                <Area 
                  type="monotone" 
                  dataKey="successRate" 
                  stroke="#82ca9d" 
                  fill="#82ca9d"
                  fillOpacity={0.3}
                />
              </AreaChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>
      </Grid>
    </Grid>
  );

  // Circuit breaker status table
  const CircuitBreakerTable = () => (
    <Card>
      <CardContent>
        <Typography variant="h6" gutterBottom>
          Circuit Breaker Status
        </Typography>
        <TableContainer component={Paper} variant="outlined">
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>Endpoint</TableCell>
                <TableCell>Status</TableCell>
                <TableCell>Failures</TableCell>
                <TableCell>Last Failure</TableCell>
                <TableCell>Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {Object.entries(circuitBreakerStatus).map(([endpoint, status]) => (
                <TableRow key={endpoint}>
                  <TableCell>{endpoint}</TableCell>
                  <TableCell>
                    <Chip
                      label={status.state.toUpperCase()}
                      color={status.state === 'closed' ? 'success' : 
                             status.state === 'half-open' ? 'warning' : 'error'}
                      size="small"
                    />
                  </TableCell>
                  <TableCell>{status.failures}</TableCell>
                  <TableCell>
                    {status.lastFailureTime > 0 
                      ? new Date(status.lastFailureTime).toLocaleTimeString()
                      : 'Never'}
                  </TableCell>
                  <TableCell>
                    {status.state !== 'closed' && (
                      <Button
                        size="small"
                        onClick={() => handleResetCircuitBreaker(endpoint)}
                      >
                        Reset
                      </Button>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      </CardContent>
    </Card>
  );

  return (
    <Box sx={{ p: 3 }}>
      {/* Header */}
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 3 }}>
        <Typography variant="h4" component="h1">
          Monitoring Dashboard
        </Typography>
        <Box sx={{ display: 'flex', gap: 2, alignItems: 'center' }}>
          <ToggleButtonGroup
            value={selectedTimeRange}
            exclusive
            onChange={(_, value) => value && setSelectedTimeRange(value)}
            size="small"
          >
            <ToggleButton value="1h">1H</ToggleButton>
            <ToggleButton value="6h">6H</ToggleButton>
            <ToggleButton value="24h">24H</ToggleButton>
          </ToggleButtonGroup>
          
          <Button
            variant={autoRefresh ? "contained" : "outlined"}
            startIcon={<RefreshIcon />}
            onClick={() => setAutoRefresh(!autoRefresh)}
          >
            Auto Refresh
          </Button>
          
          <Button
            variant="outlined"
            startIcon={<SettingsIcon />}
            onClick={() => setSettingsOpen(true)}
          >
            Settings
          </Button>
        </Box>
      </Box>

      {/* Error Alert */}
      {error && (
        <Alert severity="error" sx={{ mb: 3 }} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      {/* Health Score */}
      <Grid container spacing={3} sx={{ mb: 3 }}>
        <Grid item xs={12} md={4}>
          <HealthScoreCard />
        </Grid>
        <Grid item xs={12} md={8}>
          <Card>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                System Information
              </Typography>
              <Grid container spacing={2}>
                <Grid item xs={6}>
                  <Typography variant="body2" color="text.secondary">
                    Auto Refresh
                  </Typography>
                  <Typography variant="body1">
                    {autoRefresh ? `Enabled (${refreshInterval}ms)` : 'Disabled'}
                  </Typography>
                </Grid>
                <Grid item xs={6}>
                  <Typography variant="body2" color="text.secondary">
                    Last Refresh
                  </Typography>
                  <Typography variant="body1">
                    {lastRefresh.toLocaleTimeString()}
                  </Typography>
                </Grid>
                <Grid item xs={6}>
                  <Typography variant="body2" color="text.secondary">
                    Data Points
                  </Typography>
                  <Typography variant="body1">
                    {performanceData.length}
                  </Typography>
                </Grid>
                <Grid item xs={6}>
                  <Typography variant="body2" color="text.secondary">
                    Time Range
                  </Typography>
                  <Typography variant="body1">
                    {selectedTimeRange}
                  </Typography>
                </Grid>
              </Grid>
            </CardContent>
          </Card>
        </Grid>
      </Grid>

      {/* Metrics Overview */}
      <MetricsOverview />

      {/* Performance Charts */}
      <Box sx={{ mt: 3 }}>
        <PerformanceCharts />
      </Box>

      {/* Circuit Breaker Status */}
      <Box sx={{ mt: 3 }}>
        <CircuitBreakerTable />
      </Box>

      {/* Settings Dialog */}
      <Dialog open={settingsOpen} onClose={() => setSettingsOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>Dashboard Settings</DialogTitle>
        <DialogContent>
          <List>
            <ListItem>
              <ListItemText
                primary="Auto Refresh Interval"
                secondary={`Current: ${refreshInterval}ms`}
              />
              <Button
                variant="outlined"
                onClick={() => setRefreshInterval(prev => prev === 5000 ? 10000 : prev === 10000 ? 30000 : 5000)}
              >
                {refreshInterval === 5000 ? '5s' : refreshInterval === 10000 ? '10s' : '30s'}
              </Button>
            </ListItem>
            <Divider />
            <ListItem>
              <ListItemText
                primary="Clear Performance Data"
                secondary="Remove all historical performance data"
              />
              <Button
                variant="outlined"
                color="error"
                onClick={() => setPerformanceData([])}
              >
                Clear
              </Button>
            </ListItem>
            <Divider />
            <ListItem>
              <ListItemText
                primary="Export Metrics"
                secondary="Download current metrics as JSON"
              />
              <Button
                variant="outlined"
                onClick={() => {
                  const data = {
                    apiMetrics,
                    circuitBreakerStatus,
                    performanceData,
                    timestamp: new Date().toISOString(),
                  };
                  const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
                  const url = URL.createObjectURL(blob);
                  const a = document.createElement('a');
                  a.href = url;
                  a.download = `chatapp-metrics-${Date.now()}.json`;
                  a.click();
                  URL.revokeObjectURL(url);
                }}
              >
                Export
              </Button>
            </ListItem>
          </List>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setSettingsOpen(false)}>Close</Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
};

// Add CircularProgress import
import { CircularProgress } from '@mui/material';

export default MonitoringDashboard;
