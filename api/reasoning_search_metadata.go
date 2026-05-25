package api

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/volatiletech/sqlboiler/v4/queries/qm"

	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/mdb"
	mdbmodels "github.com/Bnei-Baruch/archive-backend/mdb/models"
	llm "github.com/Bnei-Baruch/archive-backend/search/LLM"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

type reasoningSearchResultMetadata struct {
	Title            string
	ContentType      string
	ProgramName      string
	Date             string
	OriginalLanguage string
	CollectionUID    string
	BaalSulamArticle bool
	ConnectingSource bool
}

func enrichReasoningSearchResults(db *sql.DB, uiLanguage string, results []llm.ReasoningSearchResult) error {
	if len(results) == 0 {
		return nil
	}
	if db == nil {
		return fmt.Errorf("MDB_DB is not initialized")
	}

	uiLanguage = strings.ToLower(strings.TrimSpace(uiLanguage))
	if _, ok := consts.I18N_LANG_ORDER[uiLanguage]; !ok {
		uiLanguage = consts.DEFAULT_UI_LANGUAGE
	}
	baseRequest := BaseRequest{UILanguage: uiLanguage}
	if err := inferReasoningSearchResultTypes(db, results); err != nil {
		return err
	}

	resultsByType := make(map[string][]*llm.ReasoningSearchResult)
	for i := range results {
		if results[i].ResultType == "" {
			continue
		}
		resultsByType[results[i].ResultType] = append(resultsByType[results[i].ResultType], &results[i])
	}

	loaders := []struct {
		resultType string
		load       func(*sql.DB, BaseRequest, []string) (map[string]reasoningSearchResultMetadata, error)
	}{
		{consts.ES_RESULT_TYPE_UNITS, loadReasoningSearchContentUnitMetadata},
		{consts.ES_RESULT_TYPE_COLLECTIONS, loadReasoningSearchCollectionMetadata},
		{consts.ES_RESULT_TYPE_SOURCES, loadReasoningSearchSourceMetadata},
		{consts.ES_RESULT_TYPE_TAGS, loadReasoningSearchTagMetadata},
		{consts.ES_RESULT_TYPE_BLOG_POSTS, loadReasoningSearchBlogPostMetadata},
		{consts.ES_RESULT_TYPE_TWEETS, loadReasoningSearchTweetMetadata},
	}

	for _, loader := range loaders {
		current := resultsByType[loader.resultType]
		if len(current) == 0 {
			continue
		}
		metadata, err := loader.load(db, baseRequest, reasoningSearchResultUIDs(current))
		if err != nil {
			return fmt.Errorf("failed enriching reasoning search %s metadata: %w", loader.resultType, err)
		}
		for _, result := range current {
			if meta, ok := metadata[result.MDBUID]; ok {
				result.Title = meta.Title
				result.ContentType = meta.ContentType
				result.ProgramName = meta.ProgramName
				result.Date = meta.Date
				if meta.OriginalLanguage != "" {
					result.OriginalLanguage = meta.OriginalLanguage
				}
				if meta.CollectionUID != "" {
					result.CollectionUID = meta.CollectionUID
				}
				if meta.BaalSulamArticle {
					result.BaalSulamArticle = true
				}
				if meta.ConnectingSource {
					result.ConnectingSource = true
				}
			}
		}
	}

	return nil
}

func inferReasoningSearchResultTypes(db *sql.DB, results []llm.ReasoningSearchResult) error {
	unknownUIDs := make([]string, 0, len(results))
	seenUnknown := map[string]bool{}
	for i := range results {
		results[i].MDBUID = strings.TrimSpace(results[i].MDBUID)
		results[i].ResultType = ""
		if results[i].MDBUID == "" {
			continue
		}
		if _, _, ok := parseReasoningSearchBlogPostUID(results[i].MDBUID); ok {
			results[i].ResultType = consts.ES_RESULT_TYPE_BLOG_POSTS
			continue
		}
		if !seenUnknown[results[i].MDBUID] {
			seenUnknown[results[i].MDBUID] = true
			unknownUIDs = append(unknownUIDs, results[i].MDBUID)
		}
	}
	if len(unknownUIDs) == 0 {
		return nil
	}

	typesByUID, err := loadReasoningSearchResultTypes(db, unknownUIDs)
	if err != nil {
		return fmt.Errorf("failed inferring reasoning search result types: %w", err)
	}
	for i := range results {
		if results[i].ResultType != "" {
			continue
		}
		if resultType, ok := typesByUID[results[i].MDBUID]; ok {
			results[i].ResultType = resultType
		}
	}
	return nil
}

func loadReasoningSearchResultTypes(db *sql.DB, uids []string) (map[string]string, error) {
	result := map[string]string{}
	if len(uids) == 0 {
		return result, nil
	}

	if err := setReasoningSearchResultTypesFromSources(db, uids, result); err != nil {
		return nil, err
	}
	if err := setReasoningSearchResultTypesFromContentUnits(db, uids, result); err != nil {
		return nil, err
	}
	if err := setReasoningSearchResultTypesFromCollections(db, uids, result); err != nil {
		return nil, err
	}
	if err := setReasoningSearchResultTypesFromTags(db, uids, result); err != nil {
		return nil, err
	}
	if err := setReasoningSearchResultTypesFromTweets(db, uids, result); err != nil {
		return nil, err
	}
	return result, nil
}

func setReasoningSearchResultTypesFromContentUnits(db *sql.DB, uids []string, result map[string]string) error {
	items, err := mdbmodels.ContentUnits(qm.WhereIn("uid in ?", utils.ConvertArgsString(uids)...)).All(db)
	if err != nil {
		return err
	}
	for _, item := range items {
		if _, exists := result[item.UID]; !exists {
			result[item.UID] = consts.ES_RESULT_TYPE_UNITS
		}
	}
	return nil
}

func setReasoningSearchResultTypesFromCollections(db *sql.DB, uids []string, result map[string]string) error {
	items, err := mdbmodels.Collections(qm.WhereIn("uid in ?", utils.ConvertArgsString(uids)...)).All(db)
	if err != nil {
		return err
	}
	for _, item := range items {
		if _, exists := result[item.UID]; !exists {
			result[item.UID] = consts.ES_RESULT_TYPE_COLLECTIONS
		}
	}
	return nil
}

func setReasoningSearchResultTypesFromSources(db *sql.DB, uids []string, result map[string]string) error {
	items, err := mdbmodels.Sources(qm.WhereIn("uid in ?", utils.ConvertArgsString(uids)...)).All(db)
	if err != nil {
		return err
	}
	for _, item := range items {
		if _, exists := result[item.UID]; !exists {
			result[item.UID] = consts.ES_RESULT_TYPE_SOURCES
		}
	}
	return nil
}

func setReasoningSearchResultTypesFromTags(db *sql.DB, uids []string, result map[string]string) error {
	items, err := mdbmodels.Tags(qm.WhereIn("uid in ?", utils.ConvertArgsString(uids)...)).All(db)
	if err != nil {
		return err
	}
	for _, item := range items {
		if _, exists := result[item.UID]; !exists {
			result[item.UID] = consts.ES_RESULT_TYPE_TAGS
		}
	}
	return nil
}

func setReasoningSearchResultTypesFromTweets(db *sql.DB, uids []string, result map[string]string) error {
	items, err := mdbmodels.TwitterTweets(qm.WhereIn("twitter_id in ?", utils.ConvertArgsString(uids)...)).All(db)
	if err != nil {
		return err
	}
	for _, item := range items {
		if _, exists := result[item.TwitterID]; !exists {
			result[item.TwitterID] = consts.ES_RESULT_TYPE_TWEETS
		}
	}
	return nil
}

func loadReasoningSearchContentUnitMetadata(db *sql.DB, r BaseRequest, uids []string) (map[string]reasoningSearchResultMetadata, error) {
	metadata := map[string]reasoningSearchResultMetadata{}
	if len(uids) == 0 {
		return metadata, nil
	}

	items, err := mdbmodels.ContentUnits(
		qm.WhereIn("uid in ?", utils.ConvertArgsString(uids)...),
		qm.Load("CollectionsContentUnits"),
		qm.Load("CollectionsContentUnits.Collection"),
	).All(db)
	if err != nil {
		return nil, err
	}

	ids := make([]int64, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}
	i18nsMap, err := loadCUI18ns(db, r, ids)
	if err != nil {
		return nil, err
	}

	programCollectionIDs := []int64{}
	for _, item := range items {
		for _, ccu := range item.R.CollectionsContentUnits {
			if ccu.R == nil || ccu.R.Collection == nil {
				continue
			}
			collection := ccu.R.Collection
			contentType, ok := mdb.CONTENT_TYPE_REGISTRY.ByID[collection.TypeID]
			if !ok || contentType.Name != consts.CT_VIDEO_PROGRAM {
				continue
			}
			if collection.Secure != consts.SEC_PUBLIC || !collection.Published {
				continue
			}
			programCollectionIDs = append(programCollectionIDs, collection.ID)
		}
	}
	programI18nsMap, err := loadCI18ns(db, r, utils.UniqueInt64s(programCollectionIDs))
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		contentUnit, err := mdbToCU(item)
		if err != nil {
			return nil, err
		}
		if i18ns, ok := i18nsMap[item.ID]; ok {
			setCUI18n(contentUnit, r, i18ns)
		}
		programName := ""
		programCollectionUID := ""
		for _, ccu := range item.R.CollectionsContentUnits {
			if ccu.R == nil || ccu.R.Collection == nil {
				continue
			}
			collection := ccu.R.Collection
			contentType, ok := mdb.CONTENT_TYPE_REGISTRY.ByID[collection.TypeID]
			if !ok || contentType.Name != consts.CT_VIDEO_PROGRAM {
				continue
			}
			if collection.Secure != consts.SEC_PUBLIC || !collection.Published {
				continue
			}
			program, err := mdbToC(collection)
			if err != nil {
				return nil, err
			}
			if i18ns, ok := programI18nsMap[collection.ID]; ok {
				setCI18n(program, r, i18ns)
			}
			programName = strings.TrimSpace(program.Name)
			if programName != "" {
				programCollectionUID = collection.UID
				break
			}
		}
		metadata[item.UID] = reasoningSearchResultMetadata{
			Title:            contentUnit.Name,
			ContentType:      contentUnit.ContentType,
			ProgramName:      programName,
			Date:             reasoningSearchUtilsDateString(contentUnit.FilmDate),
			OriginalLanguage: strings.TrimSpace(contentUnit.OriginalLanguage),
			CollectionUID:    programCollectionUID,
		}
	}

	return metadata, nil
}

func loadReasoningSearchCollectionMetadata(db *sql.DB, r BaseRequest, uids []string) (map[string]reasoningSearchResultMetadata, error) {
	metadata := map[string]reasoningSearchResultMetadata{}
	if len(uids) == 0 {
		return metadata, nil
	}

	items, err := mdbmodels.Collections(
		qm.WhereIn("uid in ?", utils.ConvertArgsString(uids)...),
		qm.Load("CollectionsContentUnits"),
		qm.Load("CollectionsContentUnits.ContentUnit"),
	).All(db)
	if err != nil {
		return nil, err
	}

	ids := make([]int64, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}
	i18nsMap, err := loadCI18ns(db, r, ids)
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		collection, err := mdbToC(item)
		if err != nil {
			return nil, err
		}
		if i18ns, ok := i18nsMap[item.ID]; ok {
			setCI18n(collection, r, i18ns)
		}
		date := reasoningSearchCollectionDate(item, collection)
		metadata[item.UID] = reasoningSearchResultMetadata{
			Title:       collection.Name,
			ContentType: collection.ContentType,
			Date:        reasoningSearchTimeDateString(date),
		}
	}

	return metadata, nil
}

func loadReasoningSearchSourceMetadata(db *sql.DB, r BaseRequest, uids []string) (map[string]reasoningSearchResultMetadata, error) {
	metadata := map[string]reasoningSearchResultMetadata{}
	if len(uids) == 0 {
		return metadata, nil
	}

	leafSources, err := mdbmodels.Sources(qm.WhereIn("uid in ?", utils.ConvertArgsString(uids)...)).All(db)
	if err != nil {
		return nil, err
	}

	sourcesByID := make(map[int64]*mdbmodels.Source, len(leafSources))
	leafByUID := make(map[string]*mdbmodels.Source, len(leafSources))
	pendingParents := []int64{}
	for _, source := range leafSources {
		sourcesByID[source.ID] = source
		leafByUID[source.UID] = source
		if source.ParentID.Valid {
			pendingParents = append(pendingParents, source.ParentID.Int64)
		}
	}

	for len(pendingParents) > 0 {
		parentIDs := utils.UniqueInt64s(pendingParents)
		pendingParents = nil

		missingParentIDs := make([]int64, 0, len(parentIDs))
		for _, id := range parentIDs {
			if _, ok := sourcesByID[id]; !ok {
				missingParentIDs = append(missingParentIDs, id)
			}
		}
		if len(missingParentIDs) == 0 {
			continue
		}

		parents, err := mdbmodels.Sources(qm.WhereIn("id in ?", utils.ConvertArgsInt64(missingParentIDs)...)).All(db)
		if err != nil {
			return nil, err
		}
		for _, parent := range parents {
			sourcesByID[parent.ID] = parent
			if parent.ParentID.Valid {
				pendingParents = append(pendingParents, parent.ParentID.Int64)
			}
		}
	}

	allSourceIDs := make([]int64, 0, len(sourcesByID))
	for id := range sourcesByID {
		allSourceIDs = append(allSourceIDs, id)
	}
	i18nsMap, err := loadSourceI18ns(db, r, allSourceIDs)
	if err != nil {
		return nil, err
	}

	rootIDs := []int64{}
	rootIDByLeafUID := map[string]int64{}
	for leafUID, leaf := range leafByUID {
		rootID := leaf.ID
		for current := leaf; current != nil; {
			rootID = current.ID
			if !current.ParentID.Valid {
				break
			}
			parent, ok := sourcesByID[current.ParentID.Int64]
			if !ok {
				break
			}
			current = parent
		}
		rootIDByLeafUID[leafUID] = rootID
		rootIDs = append(rootIDs, rootID)
	}
	authorNamesByRootID, err := loadReasoningSearchAuthorNamesBySourceID(db, utils.UniqueInt64s(rootIDs))
	if err != nil {
		return nil, err
	}

	for leafUID, leaf := range leafByUID {
		names := []string{}
		baalSulamArticle := false
		connectingSource := false
		for current := leaf; current != nil; {
			if current.UID == consts.SRC_ARTICLES_BAAL_SULAM {
				baalSulamArticle = true
			}
			if current.UID == consts.SRC_CONNECTING_TO_THE_SOURCE {
				connectingSource = true
			}
			names = append(names, preferredSourceI18nName(current.Name, r, i18nsMap[current.ID]))
			if !current.ParentID.Valid {
				break
			}
			parent, ok := sourcesByID[current.ParentID.Int64]
			if !ok {
				break
			}
			current = parent
		}
		for i, j := 0, len(names)-1; i < j; i, j = i+1, j-1 {
			names[i], names[j] = names[j], names[i]
		}
		title := strings.Join(names, " > ")
		if authorName := authorNamesByRootID[rootIDByLeafUID[leafUID]]; authorName != "" {
			title = authorName + " > " + title
		}
		metadata[leafUID] = reasoningSearchResultMetadata{
			Title:            title,
			ContentType:      consts.CT_SOURCE,
			BaalSulamArticle: baalSulamArticle,
			ConnectingSource: connectingSource,
		}
	}

	return metadata, nil
}

func loadReasoningSearchTagMetadata(db *sql.DB, r BaseRequest, uids []string) (map[string]reasoningSearchResultMetadata, error) {
	metadata := map[string]reasoningSearchResultMetadata{}
	if len(uids) == 0 {
		return metadata, nil
	}

	items, err := mdbmodels.Tags(qm.WhereIn("uid in ?", utils.ConvertArgsString(uids)...)).All(db)
	if err != nil {
		return nil, err
	}

	ids := make([]int64, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}
	i18nsMap, err := loadTagI18ns(db, r, ids)
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		metadata[item.UID] = reasoningSearchResultMetadata{
			Title: preferredTagI18nLabel(r, i18nsMap[item.ID]),
		}
	}

	return metadata, nil
}

func loadReasoningSearchBlogPostMetadata(db *sql.DB, _ BaseRequest, uids []string) (map[string]reasoningSearchResultMetadata, error) {
	metadata := map[string]reasoningSearchResultMetadata{}
	if len(uids) == 0 {
		return metadata, nil
	}

	byBlogID := map[int64][]int64{}
	for _, uid := range uids {
		blogID, wpID, ok := parseReasoningSearchBlogPostUID(uid)
		if !ok {
			continue
		}
		byBlogID[blogID] = append(byBlogID[blogID], wpID)
	}
	if len(byBlogID) == 0 {
		return metadata, nil
	}

	clauses := make([]string, 0, len(byBlogID))
	for blogID, wpIDs := range byBlogID {
		wpIDStrings := make([]string, len(wpIDs))
		for i, wpID := range wpIDs {
			wpIDStrings[i] = strconv.FormatInt(wpID, 10)
		}
		clauses = append(clauses, fmt.Sprintf("(blog_id = %d and wp_id in (%s))", blogID, strings.Join(wpIDStrings, ",")))
	}
	posts, err := mdbmodels.BlogPosts(qm.Where(strings.Join(clauses, " or "))).All(db)
	if err != nil {
		return nil, err
	}

	for _, post := range posts {
		mdbUID := fmt.Sprintf("%d-%d", post.BlogID, post.WPID)
		metadata[mdbUID] = reasoningSearchResultMetadata{
			Title:       post.Title,
			ContentType: consts.CT_BLOG_POST,
			Date:        post.PostedAt.Format("2006-01-02"),
		}
	}

	return metadata, nil
}

func loadReasoningSearchTweetMetadata(db *sql.DB, _ BaseRequest, uids []string) (map[string]reasoningSearchResultMetadata, error) {
	metadata := map[string]reasoningSearchResultMetadata{}
	if len(uids) == 0 {
		return metadata, nil
	}

	items, err := mdbmodels.TwitterTweets(qm.WhereIn("twitter_id in ?", utils.ConvertArgsString(uids)...)).All(db)
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		metadata[item.TwitterID] = reasoningSearchResultMetadata{
			Title: item.FullText,
			Date:  item.TweetAt.Format("2006-01-02"),
		}
	}

	return metadata, nil
}

func reasoningSearchResultUIDs(results []*llm.ReasoningSearchResult) []string {
	uids := make([]string, 0, len(results))
	seen := make(map[string]bool, len(results))
	for _, result := range results {
		uid := strings.TrimSpace(result.MDBUID)
		if uid == "" || seen[uid] {
			continue
		}
		seen[uid] = true
		uids = append(uids, uid)
	}
	return uids
}

func reasoningSearchTimeDateString(date *time.Time) string {
	if date == nil || date.IsZero() {
		return ""
	}
	return date.Format("2006-01-02")
}

func reasoningSearchUtilsDateString(date *utils.Date) string {
	if date == nil || date.IsZero() {
		return ""
	}
	return date.Format("2006-01-02")
}

func reasoningSearchCollectionDate(collection *mdbmodels.Collection, converted *Collection) *time.Time {
	var latest time.Time
	for _, ccu := range collection.R.CollectionsContentUnits {
		if ccu.R == nil || ccu.R.ContentUnit == nil {
			continue
		}
		var props mdb.ContentUnitProperties
		if err := ccu.R.ContentUnit.Properties.Unmarshal(&props); err != nil {
			continue
		}
		if !props.FilmDate.IsZero() && latest.Before(props.FilmDate.Time) {
			latest = props.FilmDate.Time
		}
	}
	if !latest.IsZero() {
		return &latest
	}
	if converted.FilmDate != nil {
		t := converted.FilmDate.Time
		return &t
	}
	if converted.StartDate != nil {
		t := converted.StartDate.Time
		return &t
	}
	if converted.EndDate != nil {
		t := converted.EndDate.Time
		return &t
	}
	return nil
}

func loadSourceI18ns(db *sql.DB, r BaseRequest, ids []int64) (map[int64]map[string]*mdbmodels.SourceI18n, error) {
	i18nsMap := make(map[int64]map[string]*mdbmodels.SourceI18n, len(ids))
	if len(ids) == 0 {
		return i18nsMap, nil
	}

	i18ns, err := mdbmodels.SourceI18ns(
		qm.WhereIn("source_id in ?", utils.ConvertArgsInt64(ids)...),
		qm.AndIn("language in ?", utils.ConvertArgsString(BaseRequestToUILanguages(r))...),
	).All(db)
	if err != nil {
		return nil, err
	}

	for _, item := range i18ns {
		byLanguage, ok := i18nsMap[item.SourceID]
		if !ok {
			byLanguage = make(map[string]*mdbmodels.SourceI18n, 1)
			i18nsMap[item.SourceID] = byLanguage
		}
		byLanguage[item.Language] = item
	}

	return i18nsMap, nil
}

func preferredSourceI18nName(fallback string, r BaseRequest, i18ns map[string]*mdbmodels.SourceI18n) string {
	for _, language := range BaseRequestToUILanguages(r) {
		if item, ok := i18ns[language]; ok && item.Name.Valid && item.Name.String != "" {
			return item.Name.String
		}
	}
	return fallback
}

func loadTagI18ns(db *sql.DB, r BaseRequest, ids []int64) (map[int64]map[string]*mdbmodels.TagI18n, error) {
	i18nsMap := make(map[int64]map[string]*mdbmodels.TagI18n, len(ids))
	if len(ids) == 0 {
		return i18nsMap, nil
	}

	i18ns, err := mdbmodels.TagI18ns(
		qm.WhereIn("tag_id in ?", utils.ConvertArgsInt64(ids)...),
		qm.AndIn("language in ?", utils.ConvertArgsString(BaseRequestToUILanguages(r))...),
	).All(db)
	if err != nil {
		return nil, err
	}

	for _, item := range i18ns {
		byLanguage, ok := i18nsMap[item.TagID]
		if !ok {
			byLanguage = make(map[string]*mdbmodels.TagI18n, 1)
			i18nsMap[item.TagID] = byLanguage
		}
		byLanguage[item.Language] = item
	}

	return i18nsMap, nil
}

func preferredTagI18nLabel(r BaseRequest, i18ns map[string]*mdbmodels.TagI18n) string {
	for _, language := range BaseRequestToUILanguages(r) {
		if item, ok := i18ns[language]; ok && item.Label.Valid && item.Label.String != "" {
			return item.Label.String
		}
	}
	return ""
}

func loadReasoningSearchAuthorNamesBySourceID(db *sql.DB, sourceIDs []int64) (map[int64]string, error) {
	result := map[int64]string{}
	if len(sourceIDs) == 0 {
		return result, nil
	}

	idStrings := make([]string, len(sourceIDs))
	for i, id := range sourceIDs {
		idStrings[i] = strconv.FormatInt(id, 10)
	}
	query := fmt.Sprintf(
		`SELECT a.name, asrc.source_id
		FROM authors a
		INNER JOIN authors_sources asrc ON a.id = asrc.author_id
		WHERE asrc.source_id IN (%s)
		ORDER BY asrc.source_id, a.id`,
		strings.Join(idStrings, ","),
	)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			name     string
			sourceID int64
		)
		if err := rows.Scan(&name, &sourceID); err != nil {
			return nil, err
		}
		if _, exists := result[sourceID]; !exists {
			result[sourceID] = name
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func parseReasoningSearchBlogPostUID(uid string) (int64, int64, bool) {
	parts := strings.SplitN(strings.TrimSpace(uid), "-", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	blogID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	wpID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	return blogID, wpID, true
}
