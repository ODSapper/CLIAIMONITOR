-- Migration: Remove chat_messages table
-- Version: 2.0.0
-- Date: 2025-12-01
-- Description: Removes supervisor web chat infrastructure. Supervisor now uses terminal I/O.

-- Drop indexes first (must happen before dropping table)
DROP INDEX IF EXISTS idx_chat_messages_time;
DROP INDEX IF EXISTS idx_chat_messages_pending;
DROP INDEX IF EXISTS idx_chat_messages_sender;
DROP INDEX IF EXISTS idx_chat_messages_repo;

-- Drop the chat_messages table
DROP TABLE IF EXISTS chat_messages;

-- Update schema version to v2
INSERT OR REPLACE INTO schema_version (version, description) VALUES
    (2, 'Removed chat_messages table - supervisor uses terminal I/O');
