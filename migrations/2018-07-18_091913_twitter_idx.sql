-- MDB generated migration file
-- rambler up

CREATE INDEX IF NOT EXISTS twitter_tweets_created_at_idx
  ON twitter_tweets
  USING BTREE (created_at);

CREATE INDEX IF NOT EXISTS twitter_tweets_user_id_tweet_at_idx
  ON twitter_tweets
  USING BTREE (user_id, tweet_at);

-- rambler down

DROP INDEX IF EXISTS twitter_tweets_created_at_idx;
DROP INDEX IF EXISTS twitter_tweets_user_id_tweet_at_idx;
