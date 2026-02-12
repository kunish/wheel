-- Migrate channel.model from comma-separated string to JSON array
-- No table rebuild needed: column type stays TEXT, only content format changes
UPDATE `channels` SET "model" = '["' || REPLACE(REPLACE("model", ', ', ','), ',', '","') || '"]' WHERE "model" != '' AND "model" NOT LIKE '[%';--> statement-breakpoint
UPDATE `channels` SET "model" = '[]' WHERE "model" = '';
