package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"gopkg.in/gin-gonic/gin.v1"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/mydb/models"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

type playlistsResponse struct {
	Playlist []*models.Playlist `json:"playlist"`
	ListResponse
}

type likesResponse struct {
	Likes []*models.Like `json:"likes"`
	ListResponse
}

type subscriptionsResponse struct {
	Subscriptions []*models.Subscription `json:"subscriptions"`
	ListResponse
}
type historyResponse struct {
	History []*models.Subscription `json:"subscriptions"`
	ListResponse
}

type subscribeRequest struct {
	Collections  []string `json:"collections" form:"collections" binding:"omitempty"`
	ContentTypes []int64  `json:"types" form:"types" binding:"omitempty"`
}

func MyPlaylistListHandler(c *gin.Context) {

	db, tx, ebd := openTransaction()
	if ebd != nil {
		utils.Must(ebd)
	}

	kcId := c.MustGet("KC_ID").(string)

	var err *HttpError
	var resp interface{}
	switch c.Request.Method {
	case http.MethodGet:
		var req ListRequest
		if c.Bind(&req) != nil {
			return
		}
		resp, err = handleGetPlaylists(tx, req, kcId)
	case http.MethodPost:
		var p models.Playlist
		if c.Bind(&p) != nil {
			return
		}
		p.AccountID = kcId
		resp, err = handleCreatePlaylist(tx, p, kcId)
	}

	closeTransaction(db, tx, err)
	concludeRequest(c, resp, err)
}

func MyPlaylistHandler(c *gin.Context) {
	id, e := strconv.ParseInt(c.Param("id"), 10, 0)
	if e != nil {
		NewBadRequestError(e).Abort(c)
		return
	}

	db, tx, ebd := openTransaction()
	if ebd != nil {
		utils.Must(ebd)
	}

	var p models.Playlist
	if c.Bind(&p) != nil {
		return
	}
	kcId := c.MustGet("KC_ID").(string)

	var err *HttpError
	var resp interface{}
	switch c.Request.Method {
	case http.MethodPatch:
		if c.Bind(&p) != nil {
			return
		}
		resp, err = handleUpdatePlaylist(tx, id, kcId, p)
	case http.MethodDelete:
		resp, err = handleDeletePlaylist(tx, id, kcId)
	}

	closeTransaction(db, tx, err)
	concludeRequest(c, resp, err)
}

func MyPlaylistItemHandler(c *gin.Context) {
	id, e := strconv.ParseInt(c.Param("id"), 10, 0)
	if e != nil {
		NewBadRequestError(e).Abort(c)
		return
	}

	db, tx, ebd := openTransaction()
	if ebd != nil {
		utils.Must(ebd)
	}

	kcId := c.MustGet("KC_ID").(string)

	var err *HttpError
	var resp interface{}
	switch c.Request.Method {
	case http.MethodPost:
		var uids []string
		if c.Bind(&uids) != nil {
			return
		}
		resp, err = handleAddToPlaylist(tx, id, uids, kcId)
	case http.MethodDelete:
		var ids []int64
		if c.Bind(&ids) != nil {
			return
		}
		resp, err = handleDeleteFromPlaylist(tx, id, ids, kcId)
	}

	closeTransaction(db, tx, err)
	concludeRequest(c, resp, err)
}

func MyLikesHandler(c *gin.Context) {
	var pr ListRequest
	if c.Bind(&pr) != nil {
		return
	}
	kcId := c.MustGet("KC_ID").(string)

	db, tx, ebd := openTransaction()
	if ebd != nil {
		utils.Must(ebd)
	}

	var err *HttpError
	var resp interface{}
	switch c.Request.Method {
	case http.MethodGet:
		var list ListRequest
		if c.Bind(&pr) != nil {
			return
		}
		resp, err = handleGetLikes(tx, kcId, list)
	case http.MethodPost:
		var uids []string
		if c.Bind(&uids) != nil {
			return
		}
		resp, err = handleAddLike(tx, uids, kcId)
	case http.MethodDelete:
		var ids []int64
		if c.Bind(&ids) != nil {
			return
		}
		resp, err = handleRemoveLikes(tx, ids, kcId)
	}

	closeTransaction(db, tx, err)
	concludeRequest(c, resp, err)
}

func MySubscriptionHandler(c *gin.Context) {
	var pr ListRequest
	if c.Bind(&pr) != nil {
		return
	}
	kcId := c.MustGet("KC_ID").(string)

	db, tx, ebd := openTransaction()
	if ebd != nil {
		utils.Must(ebd)
	}

	var err *HttpError
	var resp interface{}
	switch c.Request.Method {
	case http.MethodGet:
		var list ListRequest
		if c.Bind(&pr) != nil {
			return
		}
		resp, err = handleGetSubscriptions(tx, kcId, list)
	case http.MethodPost:
		var uids subscribeRequest
		if c.Bind(&uids) != nil {
			return
		}
		resp, err = handleSubscribe(tx, uids, kcId)
	case http.MethodDelete:
		var ids []int64
		if c.Bind(&ids) != nil {
			return
		}
		resp, err = handleUnsubscribe(tx, ids, kcId)
	}

	closeTransaction(db, tx, err)
	concludeRequest(c, resp, err)
}

func MyHistoryHandler(c *gin.Context) {
	var pr ListRequest
	if c.Bind(&pr) != nil {
		return
	}
	kcId := c.MustGet("KC_ID").(string)

	db, tx, ebd := openTransaction()
	if ebd != nil {
		utils.Must(ebd)
	}

	var err *HttpError
	var resp interface{}
	switch c.Request.Method {
	case http.MethodGet:
		var list ListRequest
		if c.Bind(&pr) != nil {
			return
		}
		resp, err = handleGetHistory(tx, kcId, list)
	case http.MethodDelete:
		var ids []int64
		if c.Bind(&ids) != nil {
			return
		}
		resp, err = handleDeleteHistory(tx, ids, kcId)
	}

	closeTransaction(db, tx, err)
	concludeRequest(c, resp, err)
}

/* HANDLERS */
func handleGetPlaylists(tx *sql.Tx, p ListRequest, kcId string) (*playlistsResponse, *HttpError) {
	mods := []qm.QueryMod{
		qm.Load("PlaylistItems"),
		qm.Where("account_id = ?", kcId),
	}
	if err := appendMyListMods(&mods, p); err != nil {
		return nil, NewBadRequestError(err)
	}
	pl, err := models.Playlists(mods...).All(tx)
	if err != nil {
		return nil, NewInternalError(err)
	}

	total, err := models.Playlists(qm.Where("account_id = ?", kcId)).Count(tx)
	if err != nil {
		return nil, NewInternalError(err)
	}

	resp := playlistsResponse{
		Playlist:     pl,
		ListResponse: ListResponse{Total: total},
	}
	return &resp, nil
}

func handleCreatePlaylist(tx *sql.Tx, p models.Playlist, kcId string) (*models.Playlist, *HttpError) {
	pl := models.Playlist{
		AccountID:  kcId,
		Name:       p.Name,
		Parameters: p.Parameters,
		Public:     p.Public,
	}
	if err := pl.Insert(tx, boil.Infer()); err != nil {
		return nil, NewInternalError(err)
	}

	return &pl, nil
}

func handleUpdatePlaylist(tx *sql.Tx, id int64, kcId string, np models.Playlist) (*models.Playlist, *HttpError) {
	p, err := models.Playlists(qm.Where("id = ?", id)).One(tx)
	if err != nil {
		return nil, NewInternalError(err)
	}

	if kcId != p.AccountID {
		err := errors.New("not acceptable")
		return nil, NewHttpError(http.StatusNotAcceptable, err, gin.ErrorTypePrivate)
	}
	if p.Name != np.Name {
		p.Name = np.Name
	}
	if p.LastPlayed != np.LastPlayed {
		p.LastPlayed = np.LastPlayed
	}
	if p.Public != np.Public {
		p.Public = np.Public
	}

	_, err = p.Update(tx, boil.Infer())
	if kcId != p.AccountID {
		return nil, NewInternalError(err)
	}
	params, err := buildNewParams(&np, p)
	if err != nil {
		return nil, NewInternalError(err)
	}
	if params != nil {
		p.Parameters = null.JSONFrom(params)
	}
	_, err = p.Update(tx, boil.Infer())
	if err != nil {
		return nil, NewInternalError(err)
	}
	return p, nil
}

func handleDeletePlaylist(tx *sql.Tx, id int64, kcId string) (*int64, *HttpError) {
	p, err := models.Playlists(qm.Where("id = ?", id)).One(tx)
	if err != nil {
		return nil, NewInternalError(err)
	}

	if kcId != p.AccountID {
		err := errors.New("not acceptable")
		return nil, NewHttpError(http.StatusNotAcceptable, err, gin.ErrorTypePrivate)
	}
	_, err = p.Delete(tx)
	if err != nil {
		return nil, NewInternalError(err)
	}
	return &p.ID, nil
}

func handleAddToPlaylist(tx *sql.Tx, id int64, uids []string, kcId string) (models.PlaylistItemSlice, *HttpError) {
	pl, err := models.Playlists(
		qm.Load("PlaylistItems"),
		qm.Where("id = ?", id),
	).One(tx)
	if err != nil {
		return nil, NewInternalError(err)
	}

	if kcId != pl.AccountID {
		err := errors.New("not acceptable")
		return nil, NewHttpError(http.StatusNotAcceptable, err, gin.ErrorTypePrivate)
	}
	hasUnit := false
	for _, x := range pl.R.PlaylistItems {
		for _, nuid := range uids {
			if x.ContentUnitUID == nuid {
				hasUnit = true
				break
			}
		}
	}
	if hasUnit {
		return nil, NewInternalError(errors.New("has unit on playlist"))
	}

	for _, nuid := range uids {
		item := models.PlaylistItem{PlaylistID: id, ContentUnitUID: nuid}
		_, err = item.Update(tx, boil.Infer())
		return nil, NewInternalError(err)
	}
	err = pl.R.PlaylistItems.ReloadAll(tx)
	if hasUnit {
		return nil, NewInternalError(err)
	}
	return pl.R.PlaylistItems, nil
}

func handleDeleteFromPlaylist(tx *sql.Tx, id int64, ids []int64, kcId string) ([]*models.PlaylistItem, *HttpError) {
	plis, err := models.PlaylistItems(
		qm.From("playlist_item as pli"),
		qm.Load("PlaylistItems"),
		qm.InnerJoin("playlist pl ON  pl.id = pli.playlist_id"),
		qm.Where("pl.account_id = ? AND pl.id = ? AND pli.id IN (?)", kcId, id, utils.ConvertArgsInt64(ids)),
	).All(tx)
	if err != nil {
		return nil, NewInternalError(err)
	}
	if _, err := plis.DeleteAll(tx); err != nil {
		return nil, NewInternalError(err)
	}

	return plis, nil
}

func handleGetLikes(tx *sql.Tx, kcId string, param ListRequest) (*likesResponse, *HttpError) {
	mods := []qm.QueryMod{qm.Where("account_id = ?", kcId)}

	total, err := models.Likes(mods...).Count(tx)
	if err != nil {
		return nil, NewInternalError(err)
	}

	if err := appendMyListMods(&mods, param); err != nil {
		return nil, NewInternalError(err)
	}
	ls, err := models.Likes(mods...).All(tx)

	if err != nil {
		return nil, NewInternalError(err)
	}

	res := likesResponse{
		Likes:        ls,
		ListResponse: ListResponse{Total: total},
	}
	return &res, nil
}

func handleAddLike(tx *sql.Tx, uids []string, kcId string) ([]*models.Like, *HttpError) {
	var likes []*models.Like
	for _, uid := range uids {
		l := models.Like{
			AccountID:      kcId,
			ContentUnitUID: uid,
		}
		if err := l.Insert(tx, boil.Infer()); err != nil {
			return nil, NewInternalError(err)
		}
		likes = append(likes, &l)
	}
	return likes, nil
}

func handleRemoveLikes(tx *sql.Tx, ids []int64, kcId string) ([]*models.Like, *HttpError) {
	ls, err := models.Likes(
		qm.WhereIn("id in (?)", utils.ConvertArgsInt64(ids)...),
		qm.Where("account_id = ?", kcId),
	).All(tx)
	if err != nil {
		return nil, NewInternalError(err)
	}

	if _, err := ls.DeleteAll(tx); err != nil {
		return nil, NewInternalError(err)
	}
	return ls, nil
}

func handleGetSubscriptions(tx *sql.Tx, kcId string, param ListRequest) (*subscriptionsResponse, *HttpError) {
	mods := []qm.QueryMod{qm.Where("account_id = ?", kcId)}

	total, err := models.Subscriptions(mods...).Count(tx)
	if err != nil {
		return nil, NewInternalError(err)
	}

	if err := appendMyListMods(&mods, param); err != nil {
		return nil, NewInternalError(err)
	}
	subs, err := models.Subscriptions(mods...).All(tx)

	if err != nil {
		return nil, NewInternalError(err)
	}

	res := subscriptionsResponse{
		Subscriptions: subs,
		ListResponse:  ListResponse{Total: total},
	}
	return &res, nil
}

func handleSubscribe(tx *sql.Tx, uids subscribeRequest, kcId string) ([]*models.Subscription, *HttpError) {
	var subs []*models.Subscription
	for _, uid := range uids.Collections {
		s := models.Subscription{
			AccountID:    kcId,
			CollectionID: null.String{String: uid, Valid: true},
		}
		if err := s.Insert(tx, boil.Infer()); err != nil {
			return nil, NewInternalError(err)
		}
		subs = append(subs, &s)
	}

	for _, id := range uids.ContentTypes {
		s := models.Subscription{
			AccountID:       kcId,
			ContentUnitType: null.Int64{Int64: id, Valid: true},
		}
		if err := s.Insert(tx, boil.Infer()); err != nil {
			return nil, NewInternalError(err)
		}
		subs = append(subs, &s)
	}
	return subs, nil
}

func handleUnsubscribe(tx *sql.Tx, ids []int64, kcId string) ([]*models.Subscription, *HttpError) {
	subs, err := models.Subscriptions(
		qm.WhereIn("id in (?)", utils.ConvertArgsInt64(ids)...),
		qm.Where("account_id = ?", kcId),
	).All(tx)
	if err != nil {
		return nil, NewInternalError(err)
	}
	if _, err := subs.DeleteAll(tx); err != nil {
		return nil, NewInternalError(err)
	}
	return subs, nil
}

func handleGetHistory(tx *sql.Tx, kcId string, param ListRequest) (*subscriptionsResponse, *HttpError) {
	mods := []qm.QueryMod{qm.Where("account_id = ?", kcId)}

	total, err := models.Subscriptions(mods...).Count(tx)
	if err != nil {
		return nil, NewInternalError(err)
	}

	if err := appendMyListMods(&mods, param); err != nil {
		return nil, NewInternalError(err)
	}
	subs, err := models.Subscriptions(mods...).All(tx)

	if err != nil {
		return nil, NewInternalError(err)
	}

	res := subscriptionsResponse{
		Subscriptions: subs,
		ListResponse:  ListResponse{Total: total},
	}
	return &res, nil
}

func handleDeleteHistory(tx *sql.Tx, ids []int64, kcId string) ([]*models.Subscription, *HttpError) {
	subs, err := models.Subscriptions(
		qm.WhereIn("id in (?)", utils.ConvertArgsInt64(ids)...),
		qm.Where("account_id = ?", kcId),
	).All(tx)
	if err != nil {
		return nil, NewInternalError(err)
	}
	if _, err := subs.DeleteAll(tx); err != nil {
		return nil, NewInternalError(err)
	}
	return subs, nil
}

/* HELPERS */

func openTransaction() (*sql.DB, *sql.Tx, error) {
	log.Info("open connection to My DB")
	db, err := sql.Open("postgres", viper.GetString("mdb.url"))
	if err != nil {
		return nil, nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, nil, err
	}

	tx, err := db.Begin()
	if err != nil {
		utils.Must(db.Close())
		return nil, nil, err
	}
	return db, tx, nil
}

func closeTransaction(db *sql.DB, tx *sql.Tx, err error) {
	log.Info("close connection to My DB")

	var eTx error
	if err == nil {
		eTx = tx.Commit()
	} else {
		eTx = tx.Rollback()
	}

	utils.Must(db.Close())
	utils.Must(eTx)
}

func appendMyListMods(mods *[]qm.QueryMod, r ListRequest) error {
	// group to remove duplicates
	*mods = append(*mods, qm.GroupBy("id"))

	if r.OrderBy != "" {
		*mods = append(*mods, qm.OrderBy(r.OrderBy))
	}

	var limit, offset int

	if r.StartIndex == 0 {
		// pagination style
		if r.PageSize == 0 {
			limit = consts.API_DEFAULT_PAGE_SIZE
		} else {
			limit = utils.Min(r.PageSize, consts.API_MAX_PAGE_SIZE)
		}
		if r.PageNumber > 1 {
			offset = (r.PageNumber - 1) * limit
		}
	} else {
		// start & stop index style for "infinite" lists
		offset = r.StartIndex - 1
		if r.StopIndex == 0 {
			limit = consts.API_MAX_PAGE_SIZE
		} else if r.StopIndex < r.StartIndex {
			return errors.New(fmt.Sprintf("Invalid range [%d-%d]", r.StartIndex, r.StopIndex))
		} else {
			limit = r.StopIndex - r.StartIndex + 1
		}
	}

	*mods = append(*mods, qm.Limit(limit))
	if offset != 0 {
		*mods = append(*mods, qm.Offset(offset))
	}

	return nil
}

func buildNewParams(newp, oldp *models.Playlist) ([]byte, error) {
	if !newp.Parameters.Valid {
		return nil, nil
	}

	var nParams map[string]interface{}
	if err := newp.Parameters.Unmarshal(&nParams); err != nil {
		return nil, NewBadRequestError(err)
	}
	if len(nParams) == 0 {
		return nil, nil
	}

	var params map[string]interface{}
	if oldp.Parameters.Valid {
		if err := oldp.Parameters.Unmarshal(&params); err != nil {
			return nil, err
		}

		for k, v := range nParams {
			params[k] = v
		}
	} else {
		params = nParams
	}

	fpa, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	return fpa, nil
}
