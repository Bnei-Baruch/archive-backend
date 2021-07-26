// Code generated by SQLBoiler 4.6.0 (https://github.com/volatiletech/sqlboiler). DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import "testing"

// This test suite runs each operation test in parallel.
// Example, if your database has 3 tables, the suite will run:
// table1, table2 and table3 Delete in parallel
// table1, table2 and table3 Insert in parallel, and so forth.
// It does NOT run each operation group in parallel.
// Separating the tests thusly grants avoidance of Postgres deadlocks.
func TestParent(t *testing.T) {
	t.Run("Histories", testHistories)
	t.Run("Likes", testLikes)
	t.Run("Playlists", testPlaylists)
	t.Run("PlaylistItems", testPlaylistItems)
	t.Run("Subscriptions", testSubscriptions)
}

func TestDelete(t *testing.T) {
	t.Run("Histories", testHistoriesDelete)
	t.Run("Likes", testLikesDelete)
	t.Run("Playlists", testPlaylistsDelete)
	t.Run("PlaylistItems", testPlaylistItemsDelete)
	t.Run("Subscriptions", testSubscriptionsDelete)
}

func TestQueryDeleteAll(t *testing.T) {
	t.Run("Histories", testHistoriesQueryDeleteAll)
	t.Run("Likes", testLikesQueryDeleteAll)
	t.Run("Playlists", testPlaylistsQueryDeleteAll)
	t.Run("PlaylistItems", testPlaylistItemsQueryDeleteAll)
	t.Run("Subscriptions", testSubscriptionsQueryDeleteAll)
}

func TestSliceDeleteAll(t *testing.T) {
	t.Run("Histories", testHistoriesSliceDeleteAll)
	t.Run("Likes", testLikesSliceDeleteAll)
	t.Run("Playlists", testPlaylistsSliceDeleteAll)
	t.Run("PlaylistItems", testPlaylistItemsSliceDeleteAll)
	t.Run("Subscriptions", testSubscriptionsSliceDeleteAll)
}

func TestExists(t *testing.T) {
	t.Run("Histories", testHistoriesExists)
	t.Run("Likes", testLikesExists)
	t.Run("Playlists", testPlaylistsExists)
	t.Run("PlaylistItems", testPlaylistItemsExists)
	t.Run("Subscriptions", testSubscriptionsExists)
}

func TestFind(t *testing.T) {
	t.Run("Histories", testHistoriesFind)
	t.Run("Likes", testLikesFind)
	t.Run("Playlists", testPlaylistsFind)
	t.Run("PlaylistItems", testPlaylistItemsFind)
	t.Run("Subscriptions", testSubscriptionsFind)
}

func TestBind(t *testing.T) {
	t.Run("Histories", testHistoriesBind)
	t.Run("Likes", testLikesBind)
	t.Run("Playlists", testPlaylistsBind)
	t.Run("PlaylistItems", testPlaylistItemsBind)
	t.Run("Subscriptions", testSubscriptionsBind)
}

func TestOne(t *testing.T) {
	t.Run("Histories", testHistoriesOne)
	t.Run("Likes", testLikesOne)
	t.Run("Playlists", testPlaylistsOne)
	t.Run("PlaylistItems", testPlaylistItemsOne)
	t.Run("Subscriptions", testSubscriptionsOne)
}

func TestAll(t *testing.T) {
	t.Run("Histories", testHistoriesAll)
	t.Run("Likes", testLikesAll)
	t.Run("Playlists", testPlaylistsAll)
	t.Run("PlaylistItems", testPlaylistItemsAll)
	t.Run("Subscriptions", testSubscriptionsAll)
}

func TestCount(t *testing.T) {
	t.Run("Histories", testHistoriesCount)
	t.Run("Likes", testLikesCount)
	t.Run("Playlists", testPlaylistsCount)
	t.Run("PlaylistItems", testPlaylistItemsCount)
	t.Run("Subscriptions", testSubscriptionsCount)
}

func TestHooks(t *testing.T) {
	t.Run("Histories", testHistoriesHooks)
	t.Run("Likes", testLikesHooks)
	t.Run("Playlists", testPlaylistsHooks)
	t.Run("PlaylistItems", testPlaylistItemsHooks)
	t.Run("Subscriptions", testSubscriptionsHooks)
}

func TestInsert(t *testing.T) {
	t.Run("Histories", testHistoriesInsert)
	t.Run("Histories", testHistoriesInsertWhitelist)
	t.Run("Likes", testLikesInsert)
	t.Run("Likes", testLikesInsertWhitelist)
	t.Run("Playlists", testPlaylistsInsert)
	t.Run("Playlists", testPlaylistsInsertWhitelist)
	t.Run("PlaylistItems", testPlaylistItemsInsert)
	t.Run("PlaylistItems", testPlaylistItemsInsertWhitelist)
	t.Run("Subscriptions", testSubscriptionsInsert)
	t.Run("Subscriptions", testSubscriptionsInsertWhitelist)
}

// TestToOne tests cannot be run in parallel
// or deadlocks can occur.
func TestToOne(t *testing.T) {
	t.Run("PlaylistItemToPlaylistUsingPlaylist", testPlaylistItemToOnePlaylistUsingPlaylist)
}

// TestOneToOne tests cannot be run in parallel
// or deadlocks can occur.
func TestOneToOne(t *testing.T) {}

// TestToMany tests cannot be run in parallel
// or deadlocks can occur.
func TestToMany(t *testing.T) {
	t.Run("PlaylistToPlaylistItems", testPlaylistToManyPlaylistItems)
}

// TestToOneSet tests cannot be run in parallel
// or deadlocks can occur.
func TestToOneSet(t *testing.T) {
	t.Run("PlaylistItemToPlaylistUsingPlaylistItems", testPlaylistItemToOneSetOpPlaylistUsingPlaylist)
}

// TestToOneRemove tests cannot be run in parallel
// or deadlocks can occur.
func TestToOneRemove(t *testing.T) {}

// TestOneToOneSet tests cannot be run in parallel
// or deadlocks can occur.
func TestOneToOneSet(t *testing.T) {}

// TestOneToOneRemove tests cannot be run in parallel
// or deadlocks can occur.
func TestOneToOneRemove(t *testing.T) {}

// TestToManyAdd tests cannot be run in parallel
// or deadlocks can occur.
func TestToManyAdd(t *testing.T) {
	t.Run("PlaylistToPlaylistItems", testPlaylistToManyAddOpPlaylistItems)
}

// TestToManySet tests cannot be run in parallel
// or deadlocks can occur.
func TestToManySet(t *testing.T) {}

// TestToManyRemove tests cannot be run in parallel
// or deadlocks can occur.
func TestToManyRemove(t *testing.T) {}

func TestReload(t *testing.T) {
	t.Run("Histories", testHistoriesReload)
	t.Run("Likes", testLikesReload)
	t.Run("Playlists", testPlaylistsReload)
	t.Run("PlaylistItems", testPlaylistItemsReload)
	t.Run("Subscriptions", testSubscriptionsReload)
}

func TestReloadAll(t *testing.T) {
	t.Run("Histories", testHistoriesReloadAll)
	t.Run("Likes", testLikesReloadAll)
	t.Run("Playlists", testPlaylistsReloadAll)
	t.Run("PlaylistItems", testPlaylistItemsReloadAll)
	t.Run("Subscriptions", testSubscriptionsReloadAll)
}

func TestSelect(t *testing.T) {
	t.Run("Histories", testHistoriesSelect)
	t.Run("Likes", testLikesSelect)
	t.Run("Playlists", testPlaylistsSelect)
	t.Run("PlaylistItems", testPlaylistItemsSelect)
	t.Run("Subscriptions", testSubscriptionsSelect)
}

func TestUpdate(t *testing.T) {
	t.Run("Histories", testHistoriesUpdate)
	t.Run("Likes", testLikesUpdate)
	t.Run("Playlists", testPlaylistsUpdate)
	t.Run("PlaylistItems", testPlaylistItemsUpdate)
	t.Run("Subscriptions", testSubscriptionsUpdate)
}

func TestSliceUpdateAll(t *testing.T) {
	t.Run("Histories", testHistoriesSliceUpdateAll)
	t.Run("Likes", testLikesSliceUpdateAll)
	t.Run("Playlists", testPlaylistsSliceUpdateAll)
	t.Run("PlaylistItems", testPlaylistItemsSliceUpdateAll)
	t.Run("Subscriptions", testSubscriptionsSliceUpdateAll)
}
