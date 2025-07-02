-- path: birdlens-be/cmd/migrate/migrations/000001_init.up.sql
-- (This is the complete, corrected content for this file)
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE IF NOT EXISTS users (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  username varchar(20) NOT NULL,
  age int,
  first_name varchar(50) NOT NULL,
  last_name varchar(50) NOT NULL,
  email citext UNIQUE NOT NULL,
  hashed_password bytea NOT NULL,
  created_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
  updated_at timestamp(0) with time zone
);

CREATE TABLE tags (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name TEXT NOT NULL
);

CREATE TABLE groups (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name TEXT NOT NULL
);

CREATE TABLE location (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  code TEXT NOT NULL,
  name TEXT NOT NULL
);

-- This is the original, correct state for the initial migration.
-- The UNIQUE constraint will be added in a later migration file.
CREATE TABLE roles (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name VARCHAR(50) NOT NULL
);

CREATE TABLE categories (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT
);

CREATE TABLE posts (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  group_id BIGINT,
  location_name VARCHAR(100),
  latitude DECIMAL(10,8),
  longitude DECIMAL(10,8),
  privacy_level VARCHAR(20),
  type TEXT,
  is_featured BOOLEAN,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP,
  FOREIGN KEY (group_id) REFERENCES groups(id)
);

CREATE TABLE media (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  post_id BIGINT NOT NULL,
  media_url TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (post_id) REFERENCES posts(id)
);

CREATE TABLE group_posts (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  group_id BIGINT NOT NULL,
  post_id BIGINT NOT NULL,
  FOREIGN KEY (group_id) REFERENCES groups(id),
  FOREIGN KEY (post_id) REFERENCES posts(id)
);

CREATE TABLE events (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  location_id BIGINT NOT NULL,
  title VARCHAR(255) NOT NULL,
  description TEXT,
  cover_photo_url TEXT,
  start_date TIMESTAMP NOT NULL,
  end_date TIMESTAMP NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP,
  FOREIGN KEY (location_id) REFERENCES location(id)
);

CREATE TABLE tours (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  event_id BIGINT NOT NULL,
  price DECIMAL(10,2) NOT NULL,
  capacity INT NOT NULL,
  FOREIGN KEY (event_id) REFERENCES events(id)
);

CREATE TABLE tour_attendees (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  tour_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  user_status VARCHAR(50) NOT NULL,
  registered_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (tour_id) REFERENCES tours(id),
  FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE comments (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  post_id BIGINT NOT NULL,
  content TEXT NOT NULL,
  FOREIGN KEY (post_id) REFERENCES posts(id)
);

CREATE TABLE user_roles (
  user_id BIGINT NOT NULL,
  role_id BIGINT NOT NULL,
  PRIMARY KEY (user_id, role_id),
  FOREIGN KEY (user_id) REFERENCES users(id),
  FOREIGN KEY (role_id) REFERENCES roles(id)
);

CREATE TABLE user_stock (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_id BIGINT NOT NULL,
  FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE items (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_stock_id BIGINT NOT NULL,
  price DECIMAL(10,2) NOT NULL,
  FOREIGN KEY (user_stock_id) REFERENCES user_stock(id)
);

CREATE TABLE item_categories (
  item_id BIGINT NOT NULL,
  category_id BIGINT NOT NULL,
  PRIMARY KEY (item_id, category_id),
  FOREIGN KEY (item_id) REFERENCES items(id),
  FOREIGN KEY (category_id) REFERENCES categories(id)
);

CREATE TABLE post_tags (
  post_id BIGINT NOT NULL,
  tag_id BIGINT NOT NULL,
  PRIMARY KEY (post_id, tag_id),
  FOREIGN KEY (post_id) REFERENCES posts(id),
  FOREIGN KEY (tag_id) REFERENCES tags(id)
);