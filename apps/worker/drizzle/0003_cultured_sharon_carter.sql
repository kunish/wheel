ALTER TABLE `llm_prices` ADD `cache_read_price` real DEFAULT 0 NOT NULL;--> statement-breakpoint
ALTER TABLE `llm_prices` ADD `cache_write_price` real DEFAULT 0 NOT NULL;