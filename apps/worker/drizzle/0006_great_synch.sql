ALTER TABLE `groups` ADD `session_keep_time` integer DEFAULT 0 NOT NULL;--> statement-breakpoint
ALTER TABLE `relay_logs` DROP COLUMN `successful_round`;