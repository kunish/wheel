CREATE TABLE `llm_prices` (
	`id` integer PRIMARY KEY AUTOINCREMENT NOT NULL,
	`name` text NOT NULL,
	`input_price` real DEFAULT 0 NOT NULL,
	`output_price` real DEFAULT 0 NOT NULL,
	`source` text DEFAULT 'manual' NOT NULL,
	`created_at` text DEFAULT (datetime('now')),
	`updated_at` text DEFAULT (datetime('now'))
);
--> statement-breakpoint
CREATE UNIQUE INDEX `llm_prices_name_unique` ON `llm_prices` (`name`);