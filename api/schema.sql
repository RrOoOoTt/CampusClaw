CREATE TABLE IF NOT EXISTS classes (id BIGINT PRIMARY KEY AUTO_INCREMENT, name VARCHAR(100) NOT NULL UNIQUE);
CREATE TABLE IF NOT EXISTS users (id BIGINT PRIMARY KEY AUTO_INCREMENT, username VARCHAR(100) NOT NULL UNIQUE, password_hash VARCHAR(100) NOT NULL, role ENUM('teacher','student') NOT NULL, class_id BIGINT NOT NULL, FOREIGN KEY (class_id) REFERENCES classes(id));
CREATE TABLE IF NOT EXISTS sessions (token_hash BINARY(32) PRIMARY KEY, user_id BIGINT NOT NULL, expires_at DATETIME NOT NULL, FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE);
CREATE TABLE IF NOT EXISTS materials (id BIGINT PRIMARY KEY AUTO_INCREMENT, class_id BIGINT NOT NULL, uploader_id BIGINT NOT NULL, title VARCHAR(255) NOT NULL, storage_name VARCHAR(100) NOT NULL UNIQUE, size_bytes BIGINT NOT NULL, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY (class_id) REFERENCES classes(id), FOREIGN KEY (uploader_id) REFERENCES users(id));
CREATE TABLE IF NOT EXISTS knowledge_entries (material_id BIGINT PRIMARY KEY, body MEDIUMTEXT NOT NULL, FOREIGN KEY (material_id) REFERENCES materials(id) ON DELETE CASCADE);
CREATE TABLE IF NOT EXISTS knowledge_chunks (
 id BIGINT PRIMARY KEY AUTO_INCREMENT,
 class_id BIGINT NOT NULL,
 material_id BIGINT NOT NULL,
 knowledge_entry_id BIGINT NOT NULL,
 chunk_index INT NOT NULL,
 start_char INT NOT NULL,
 end_char INT NOT NULL,
 chunk_text TEXT NOT NULL,
 index_status ENUM('pending','ready','failed') NOT NULL DEFAULT 'pending',
 INDEX idx_class_status (class_id,index_status),
 UNIQUE KEY uniq_material_chunk (material_id,chunk_index),
 FULLTEXT KEY idx_chunk_text (chunk_text) WITH PARSER ngram,
 FOREIGN KEY (class_id) REFERENCES classes(id),
 FOREIGN KEY (material_id) REFERENCES materials(id) ON DELETE CASCADE,
 FOREIGN KEY (knowledge_entry_id) REFERENCES knowledge_entries(material_id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS assistant_settings (
 class_id BIGINT PRIMARY KEY,
 system_prompt TEXT NOT NULL,
 guidance_enabled BOOLEAN NOT NULL DEFAULT TRUE,
 updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
 FOREIGN KEY (class_id) REFERENCES classes(id)
);
CREATE TABLE IF NOT EXISTS assistant_conversations (
 id BIGINT PRIMARY KEY AUTO_INCREMENT,
 user_id BIGINT NOT NULL,
 class_id BIGINT NOT NULL,
 title VARCHAR(100) NOT NULL DEFAULT '新会话',
 busy BOOLEAN NOT NULL DEFAULT FALSE,
 created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
 INDEX idx_assistant_owner (user_id,class_id,updated_at),
 FOREIGN KEY (user_id) REFERENCES users(id),
 FOREIGN KEY (class_id) REFERENCES classes(id)
);
CREATE TABLE IF NOT EXISTS assistant_messages (
 id BIGINT PRIMARY KEY AUTO_INCREMENT,
 conversation_id BIGINT NOT NULL,
 role ENUM('user','assistant') NOT NULL,
 content MEDIUMTEXT NOT NULL,
 citations JSON NOT NULL,
 prompt_snapshot TEXT NOT NULL,
 guidance_enabled BOOLEAN NOT NULL,
 state ENUM('streaming','complete','interrupted','error') NOT NULL,
 created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
 INDEX idx_assistant_message (conversation_id,id),
 FOREIGN KEY (conversation_id) REFERENCES assistant_conversations(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS material_cleanup_jobs (
 material_id BIGINT PRIMARY KEY,
 class_id BIGINT NOT NULL,
 storage_name VARCHAR(100) NOT NULL,
 created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
