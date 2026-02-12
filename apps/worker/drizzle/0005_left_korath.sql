PRAGMA foreign_keys=OFF;--> statement-breakpoint
CREATE TABLE `__new_groups` (
	`id` integer PRIMARY KEY AUTOINCREMENT NOT NULL,
	`name` text NOT NULL,
	`mode` integer DEFAULT 1 NOT NULL,
	`first_token_time_out` integer DEFAULT 0 NOT NULL,
	`order` integer DEFAULT 0 NOT NULL
);
--> statement-breakpoint
INSERT INTO `__new_groups`("id", "name", "mode", "first_token_time_out", "order") SELECT "id", "name", "mode", "first_token_time_out", "order" FROM `groups`;--> statement-breakpoint
DROP TABLE `groups`;--> statement-breakpoint
ALTER TABLE `__new_groups` RENAME TO `groups`;--> statement-breakpoint
PRAGMA foreign_keys=ON;--> statement-breakpoint
ALTER TABLE `channels` DROP COLUMN `match_regex`;