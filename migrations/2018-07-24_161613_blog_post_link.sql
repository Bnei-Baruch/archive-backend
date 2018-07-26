-- MDB generated migration file
-- rambler up

ALTER TABLE blog_posts
  ADD COLUMN link varchar(255) NOT NULL DEFAULT '',
  ADD COLUMN filtered boolean NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS blog_posts_link_idx
  ON blog_posts
  USING BTREE (link);

-- rambler down

DROP INDEX IF EXISTS blog_posts_link_idx;

ALTER TABLE blog_posts
  drop column filtered,
  drop column link;

