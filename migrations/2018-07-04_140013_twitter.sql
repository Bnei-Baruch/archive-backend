-- MDB generated migration file
-- rambler up

DROP TABLE IF EXISTS twitter_users;
CREATE TABLE twitter_users (
  id           BIGSERIAL PRIMARY KEY,
  username     VARCHAR(30) UNIQUE NOT NULL,
  account_id   VARCHAR(16) UNIQUE NOT NULL,
  display_name VARCHAR(50)        NOT NULL
);

DROP TABLE IF EXISTS twitter_tweets;
CREATE TABLE twitter_tweets (
  id         BIGSERIAL PRIMARY KEY,
  user_id    BIGINT REFERENCES twitter_users (id)          NOT NULL,
  twitter_id VARCHAR(64) UNIQUE                            NOT NULL,
  full_text  TEXT                                          NOT NULL,
  tweet_at   TIMESTAMP WITH TIME ZONE                      NOT NULL,
  raw        JSONB                                         NULL,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT now_utc()    NOT NULL
);

insert into twitter_users (username, account_id, display_name) values
  ('Michael_Laitman','27005015','Михаэль Лайтман'),
  ('laitman_co_il','28344482','מיכאל לייטמן');

-- rambler down

DROP TABLE IF EXISTS twitter_users;
DROP TABLE IF EXISTS twitter_tweets;
