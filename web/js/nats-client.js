// NATS WebSocket Client for Dashboard
// Handles real-time communication with Captain and agents via NATS

class NATSClient {
    constructor() {
        this.nc = null;
        this.subscriptions = [];
        this.connected = false;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 10;
        this.reconnectDelay = 1000;
        this.messageCallbacks = [];
        this.presenceCallbacks = [];

        console.log('[NATS-CLIENT] NATSClient initialized');
    }

    /**
     * Connect to NATS via WebSocket
     * @returns {Promise<void>}
     */
    async connect() {
        try {
            console.log('[NATS-CLIENT] Connecting to NATS WebSocket at ws://localhost:4223...');

            // Check if nats is available
            if (typeof nats === 'undefined') {
                throw new Error('nats.ws library not loaded');
            }

            // Connect to NATS WebSocket
            this.nc = await nats.connect({
                servers: 'ws://localhost:4223',
                name: 'dashboard-client',
                maxReconnectAttempts: this.maxReconnectAttempts,
                reconnectTimeWait: this.reconnectDelay,
            });

            this.connected = true;
            this.reconnectAttempts = 0;
            console.log('[NATS-CLIENT] Connected to NATS successfully');

            // Set up default subscriptions
            await this.setupDefaultSubscriptions();

            // Handle connection closed
            (async () => {
                for await (const status of this.nc.status()) {
                    console.log('[NATS-CLIENT] Status:', status.type, status.data);

                    if (status.type === 'disconnect') {
                        this.connected = false;
                        this.handleDisconnect();
                    } else if (status.type === 'reconnect') {
                        this.connected = true;
                        console.log('[NATS-CLIENT] Reconnected to NATS');
                        await this.setupDefaultSubscriptions();
                    }
                }
            })().catch(err => {
                console.error('[NATS-CLIENT] Status monitoring error:', err);
            });

        } catch (error) {
            console.error('[NATS-CLIENT] Connection failed:', error);
            this.connected = false;
            this.scheduleReconnect();
            throw error;
        }
    }

    /**
     * Set up default subscriptions for chat and presence
     */
    async setupDefaultSubscriptions() {
        try {
            // Subscribe to chat messages directed at dashboard
            await this.subscribe('chat.dashboard', (msg) => {
                console.log('[NATS-CLIENT] Received chat message:', msg);
                this.notifyMessageCallbacks(msg);
            });

            // Subscribe to presence updates (all agents)
            await this.subscribe('presence.>', (msg) => {
                console.log('[NATS-CLIENT] Received presence update:', msg);
                this.notifyPresenceCallbacks(msg);
            });

            console.log('[NATS-CLIENT] Default subscriptions set up');
        } catch (error) {
            console.error('[NATS-CLIENT] Failed to set up default subscriptions:', error);
        }
    }

    /**
     * Handle disconnection
     */
    handleDisconnect() {
        console.log('[NATS-CLIENT] Disconnected from NATS');
        this.scheduleReconnect();
    }

    /**
     * Schedule reconnection attempt
     */
    scheduleReconnect() {
        if (this.reconnectAttempts >= this.maxReconnectAttempts) {
            console.error('[NATS-CLIENT] Max reconnect attempts reached');
            return;
        }

        this.reconnectAttempts++;
        const delay = Math.min(this.reconnectDelay * Math.pow(2, this.reconnectAttempts), 30000);
        console.log(`[NATS-CLIENT] Scheduling reconnect in ${delay}ms (attempt ${this.reconnectAttempts})`);

        setTimeout(() => {
            this.connect().catch(err => {
                console.error('[NATS-CLIENT] Reconnect failed:', err);
            });
        }, delay);
    }

    /**
     * Disconnect from NATS
     */
    async disconnect() {
        if (!this.nc) return;

        try {
            console.log('[NATS-CLIENT] Disconnecting from NATS...');

            // Unsubscribe from all subscriptions
            for (const sub of this.subscriptions) {
                try {
                    await sub.unsubscribe();
                } catch (err) {
                    console.error('[NATS-CLIENT] Error unsubscribing:', err);
                }
            }
            this.subscriptions = [];

            // Close connection
            await this.nc.close();
            this.nc = null;
            this.connected = false;
            console.log('[NATS-CLIENT] Disconnected successfully');
        } catch (error) {
            console.error('[NATS-CLIENT] Error during disconnect:', error);
        }
    }

    /**
     * Subscribe to a NATS subject
     * @param {string} subject - NATS subject to subscribe to
     * @param {Function} callback - Callback function to handle messages
     * @returns {Promise<void>}
     */
    async subscribe(subject, callback) {
        if (!this.nc) {
            throw new Error('Not connected to NATS');
        }

        try {
            const sub = this.nc.subscribe(subject);
            this.subscriptions.push(sub);
            console.log(`[NATS-CLIENT] Subscribed to ${subject}`);

            // Process messages
            (async () => {
                for await (const msg of sub) {
                    try {
                        const data = this.decode(msg.data);
                        callback(data);
                    } catch (err) {
                        console.error(`[NATS-CLIENT] Error processing message on ${subject}:`, err);
                    }
                }
            })().catch(err => {
                console.error(`[NATS-CLIENT] Subscription error on ${subject}:`, err);
            });

        } catch (error) {
            console.error(`[NATS-CLIENT] Failed to subscribe to ${subject}:`, error);
            throw error;
        }
    }

    /**
     * Publish a message to a NATS subject
     * @param {string} subject - NATS subject to publish to
     * @param {Object} data - Message data
     * @returns {Promise<void>}
     */
    async publish(subject, data) {
        if (!this.nc) {
            throw new Error('Not connected to NATS');
        }

        try {
            const encoded = this.encode(data);
            this.nc.publish(subject, encoded);
            console.log(`[NATS-CLIENT] Published to ${subject}:`, data);
        } catch (error) {
            console.error(`[NATS-CLIENT] Failed to publish to ${subject}:`, error);
            throw error;
        }
    }

    /**
     * Send a message to Captain
     * @param {string} text - Message text
     * @returns {Promise<void>}
     */
    async sendToCaptain(text) {
        const message = {
            id: 'msg-' + Date.now(),
            from: 'dashboard',
            text: text,
            timestamp: Date.now()
        };

        await this.publish('chat.captain', message);
        console.log('[NATS-CLIENT] Sent message to Captain:', text);
    }

    /**
     * Send a message to a specific agent
     * @param {string} agentId - Agent ID
     * @param {string} text - Message text
     * @returns {Promise<void>}
     */
    async sendToAgent(agentId, text) {
        const message = {
            id: 'msg-' + Date.now(),
            from: 'dashboard',
            text: text,
            timestamp: Date.now()
        };

        await this.publish(`chat.agent.${agentId}`, message);
        console.log(`[NATS-CLIENT] Sent message to agent ${agentId}:`, text);
    }

    /**
     * Register a callback for incoming chat messages
     * @param {Function} callback - Callback function
     */
    onMessage(callback) {
        this.messageCallbacks.push(callback);
    }

    /**
     * Register a callback for presence updates
     * @param {Function} callback - Callback function
     */
    onPresence(callback) {
        this.presenceCallbacks.push(callback);
    }

    /**
     * Notify all message callbacks
     * @param {Object} message - Message data
     */
    notifyMessageCallbacks(message) {
        for (const callback of this.messageCallbacks) {
            try {
                callback(message);
            } catch (err) {
                console.error('[NATS-CLIENT] Error in message callback:', err);
            }
        }
    }

    /**
     * Notify all presence callbacks
     * @param {Object} presence - Presence data
     */
    notifyPresenceCallbacks(presence) {
        for (const callback of this.presenceCallbacks) {
            try {
                callback(presence);
            } catch (err) {
                console.error('[NATS-CLIENT] Error in presence callback:', err);
            }
        }
    }

    /**
     * Encode data to Uint8Array for NATS
     * @param {Object} data - Data to encode
     * @returns {Uint8Array}
     */
    encode(data) {
        const jsonString = JSON.stringify(data);
        const encoder = new TextEncoder();
        return encoder.encode(jsonString);
    }

    /**
     * Decode Uint8Array from NATS to object
     * @param {Uint8Array} data - Data to decode
     * @returns {Object}
     */
    decode(data) {
        const decoder = new TextDecoder();
        const jsonString = decoder.decode(data);
        return JSON.parse(jsonString);
    }

    /**
     * Check if connected to NATS
     * @returns {boolean}
     */
    isConnected() {
        return this.connected && this.nc !== null;
    }

    /**
     * Get connection status information
     * @returns {Object}
     */
    getStatus() {
        return {
            connected: this.connected,
            reconnectAttempts: this.reconnectAttempts,
            subscriptions: this.subscriptions.length
        };
    }
}

// Export to global scope
window.NATSClient = NATSClient;
