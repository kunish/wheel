CREATE TABLE `api_keys` (
	`id` integer PRIMARY KEY AUTOINCREMENT NOT NULL,
	`name` text NOT NULL,
	`api_key` text NOT NULL,
	`enabled` integer DEFAULT true NOT NULL,
	`expire_at` integer DEFAULT 0 NOT NULL,
	`max_cost` real DEFAULT 0 NOT NULL,
	`supported_models` text DEFAULT '' NOT NULL,
	`total_cost` real DEFAULT 0 NOT NULL
);
--> statement-breakpoint
CREATE UNIQUE INDEX `api_keys_api_key_unique` ON `api_keys` (`api_key`);--> statement-breakpoint
CREATE TABLE `channel_keys` (
	`id` integer PRIMARY KEY AUTOINCREMENT NOT NULL,
	`channel_id` integer NOT NULL,
	`enabled` integer DEFAULT true NOT NULL,
	`channel_key` text NOT NULL,
	`status_code` integer DEFAULT 0 NOT NULL,
	`last_use_timestamp` integer DEFAULT 0 NOT NULL,
	`total_cost` real DEFAULT 0 NOT NULL,
	`remark` text DEFAULT '' NOT NULL,
	FOREIGN KEY (`channel_id`) REFERENCES `channels`(`id`) ON UPDATE no action ON DELETE cascade
);
--> statement-breakpoint
CREATE TABLE `channels` (
	`id` integer PRIMARY KEY AUTOINCREMENT NOT NULL,
	`name` text NOT NULL,
	`type` integer DEFAULT 1 NOT NULL,
	`enabled` integer DEFAULT true NOT NULL,
	`base_urls` text DEFAULT '[]' NOT NULL,
	`model` text DEFAULT '' NOT NULL,
	`custom_model` text DEFAULT '' NOT NULL,
	`proxy` integer DEFAULT false NOT NULL,
	`auto_sync` integer DEFAULT false NOT NULL,
	`auto_group` integer DEFAULT 0 NOT NULL,
	`custom_header` text DEFAULT '[]' NOT NULL,
	`param_override` text,
	`channel_proxy` text,
	`match_regex` text
);
--> statement-breakpoint
CREATE UNIQUE INDEX `channels_name_unique` ON `channels` (`name`);--> statement-breakpoint
CREATE TABLE `group_items` (
	`id` integer PRIMARY KEY AUTOINCREMENT NOT NULL,
	`group_id` integer NOT NULL,
	`channel_id` integer NOT NULL,
	`model_name` text DEFAULT '' NOT NULL,
	`priority` integer DEFAULT 0 NOT NULL,
	`weight` integer DEFAULT 1 NOT NULL,
	FOREIGN KEY (`group_id`) REFERENCES `groups`(`id`) ON UPDATE no action ON DELETE cascade,
	FOREIGN KEY (`channel_id`) REFERENCES `channels`(`id`) ON UPDATE no action ON DELETE cascade
);
--> statement-breakpoint
CREATE TABLE `groups` (
	`id` integer PRIMARY KEY AUTOINCREMENT NOT NULL,
	`name` text NOT NULL,
	`mode` integer DEFAULT 1 NOT NULL,
	`match_regex` text DEFAULT '' NOT NULL,
	`first_token_time_out` integer DEFAULT 30 NOT NULL
);
--> statement-breakpoint
CREATE TABLE `relay_logs` (
	`id` integer PRIMARY KEY NOT NULL,
	`time` integer NOT NULL,
	`request_model_name` text DEFAULT '' NOT NULL,
	`channel_id` integer DEFAULT 0 NOT NULL,
	`channel_name` text DEFAULT '' NOT NULL,
	`actual_model_name` text DEFAULT '' NOT NULL,
	`input_tokens` integer DEFAULT 0 NOT NULL,
	`output_tokens` integer DEFAULT 0 NOT NULL,
	`ftut` integer DEFAULT 0 NOT NULL,
	`use_time` integer DEFAULT 0 NOT NULL,
	`cost` real DEFAULT 0 NOT NULL,
	`request_content` text DEFAULT '' NOT NULL,
	`response_content` text DEFAULT '' NOT NULL,
	`error` text DEFAULT '' NOT NULL,
	`attempts` text DEFAULT '[]' NOT NULL,
	`total_attempts` integer DEFAULT 0 NOT NULL,
	`successful_round` integer DEFAULT 0 NOT NULL
);
--> statement-breakpoint
CREATE TABLE `settings` (
	`key` text PRIMARY KEY NOT NULL,
	`value` text DEFAULT '' NOT NULL
);
--> statement-breakpoint
CREATE TABLE `users` (
	`id` integer PRIMARY KEY AUTOINCREMENT NOT NULL,
	`username` text NOT NULL,
	`password` text NOT NULL
);
--> statement-breakpoint
CREATE UNIQUE INDEX `users_username_unique` ON `users` (`username`);