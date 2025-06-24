-- Add UUID column with generated default
ALTER TABLE users ADD COLUMN uuid UUID DEFAULT gen_random_uuid();