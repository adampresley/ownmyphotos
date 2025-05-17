--
-- Settings
--
CREATE TABLE IF NOT EXISTS "settings" (
   id integer PRIMARY KEY,
   max_workers integer default 5,
   collector_schedule text,
   library_path text,
   thumbnail_size integer
);

--
-- People
--
CREATE TABLE IF NOT EXISTS "people" (
   id integer PRIMARY KEY AUTOINCREMENT,
   created_at datetime,
   updated_at datetime,
   deleted_at datetime,
   name text unique
);

CREATE INDEX IF NOT EXISTS idx_people_deleted_at ON people (deleted_at);

--
-- Keywords
--
CREATE TABLE IF NOT EXISTS "keywords" (
   keyword text PRIMARY KEY
);

--
-- Folders 
-- 
CREATE TABLE IF NOT EXISTS "folders" (
   full_path text,
   folder_name text,
   parent_path text,
   key_photo_id text,

   PRIMARY KEY(full_path)
);

--
-- Photos
--
CREATE TABLE IF NOT EXISTS "photos" (
   id text PRIMARY KEY,
   created_at datetime,
   updated_at datetime,
   deleted_at datetime,
   file_name text,
   ext text,
   full_path text,
   metadata_hash text,
   lens_make text,
   lens_model text,
   lens_id text,
   make text,
   model text,
   caption text,
   title text,
   creation_date_time datetime,
   width integer,
   height integer,
   latitude real,
   longitude real,
   iptc_digest text,
   year text
);

CREATE INDEX IF NOT EXISTS idx_photos_deleted_at ON photos (deleted_at);
CREATE INDEX IF NOT EXISTS idx_photos_caption ON photos (caption);
CREATE INDEX IF NOT EXISTS idx_photos_title ON photos (title);
CREATE INDEX IF NOT EXISTS idx_photos_year ON photos (year);
CREATE INDEX IF NOT EXISTS idx_photos_full_path ON photos (full_path);

--
-- Associations to photos
--
CREATE TABLE IF NOT EXISTS "photos_keywords" (
   photo_id text,
   keyword text,
   PRIMARY KEY (photo_id, keyword)
);

CREATE TABLE IF NOT EXISTS "photos_people" (
   photo_id text,
   person_id integer,
   PRIMARY KEY (photo_id, person_id)
);

