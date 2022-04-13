package tests

/*
type TagDashboardSuite struct {
	suite.Suite
	utils.TestDBManager
	Cache cache.CacheManager
	ctx   context.Context
}

func (s *TagDashboardSuite) SetupSuite() {
	utils.InitConfig("", "../")
	err := s.InitTestDB()
	if err != nil {
		panic(err)
	}
	s.ctx = context.Background()
	utils.Must(mdb.InitTypeRegistries(s.DB))

	refreshIntervals := map[string]time.Duration{"SearchStats": 5 * time.Minute}
	s.Cache = cache.NewCacheManagerImpl(s.DB, refreshIntervals)
	common.InitWithDefault(s.DB, &s.Cache)
	// Set package db and esc variables.
	boil.DebugMode = viper.GetString("boiler-mode") == "debug"
}

func (s *TagDashboardSuite) TearDownSuite() {
	// Close connections.
	common.Shutdown()
	// Drop test database.
	s.Require().Nil(s.DestroyTestDB())
}

func TestTagDashboard(t *testing.T) {
	suite.Run(t, new(TagDashboardSuite))
}

func (s *TagDashboardSuite) TestMediaPagination() {
	boil.DebugMode = true
	tags := s.mkTags(1)

	cusLessons := s.mkCUs(9, mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_LESSON_PART].ID, tags)
	cusPrograms := s.mkCUs(28, mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_VIDEO_PROGRAM_CHAPTER].ID, tags)
	labels := s.mkLabels(20, "media", tags)

	pageSize := 18
	total := len(cusLessons) + len(cusPrograms) + len(labels)

	for i := 1; pageSize*(i-1) <= total; i++ {
		if i != 3 {
			continue
		}
		r := api.TagDashboardRequest{
			ListRequest: api.ListRequest{PageNumber: i, PageSize: pageSize, BaseRequest: api.BaseRequest{Language: "he"}},
			TagsFilter:  api.TagsFilter{Tags: []string{tags[0].UID}},
		}

		resp, err := api.HandleTagDashboard(common.CACHE, s.DB, r)
		s.Nil(err)
		count := pageSize
		if i*pageSize > total {
			count = total % pageSize
		}
		s.EqualValues(count, len(resp.Items))
		s.EqualValues(total, resp.MediaTotal)

		s.Nil(err)
		byType := s.respCusByCT(resp)
		expLesson := 0
		expProg := 0
		expLabels := 0
		if i == 1 {
			expLesson = 6
			expProg = 6
			expLabels = 6
		} else if i == 2 {
			expLesson = 3
			expProg = 8
			expLabels = 7
		} else if i == 3 {
			expLesson = 0
			expProg = 11
			expLabels = 7
		} else if i == 4 {
			expLesson = 0
			expProg = 3
			expLabels = 0
		}
		s.EqualValues(expLesson, len(byType[mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_LESSON_PART].ID]), fmt.Sprintf("number of LESSONS, Page n %d", i))
		s.EqualValues(expProg, len(byType[mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_VIDEO_PROGRAM_CHAPTER].ID]), fmt.Sprintf("number of PROGRAMS, Page n %d", i))
		s.EqualValues(expLabels, len(byType[-2]), fmt.Sprintf("number of labels, Page n %d", i))
	}
}

func (s *TagDashboardSuite) TestMediaPaginationExtraOffset() {
	boil.DebugMode = true
	tags := s.mkTags(1)

	cusLessons := s.mkCUs(4, mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_LESSON_PART].ID, tags)
	cusPrograms := s.mkCUs(28, mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_VIDEO_PROGRAM_CHAPTER].ID, tags)
	labels := s.mkLabels(20, "media", tags)

	pageSize := 10
	total := len(cusLessons) + len(cusPrograms) + len(labels)

	for i := 1; pageSize*(i-1) <= total; i++ {
		r := api.TagDashboardRequest{
			ListRequest: api.ListRequest{PageNumber: i, PageSize: pageSize, BaseRequest: api.BaseRequest{Language: "he"}},
			TagsFilter:  api.TagsFilter{Tags: []string{tags[0].UID}},
		}

		resp, err := api.HandleTagDashboard(common.CACHE, s.DB, r)
		s.Nil(err)
		count := pageSize
		if i*pageSize > total {
			count = total % pageSize
		}
		s.EqualValues(count, len(resp.Items))
		s.EqualValues(total, resp.MediaTotal)

		s.Nil(err)
		byType := s.respCusByCT(resp)
		expLesson := 0
		expProg := 0
		expLabels := 0
		if i == 1 {
			expLesson = 6
			expProg = 6
			expLabels = 6
		} else if i == 2 {
			expLesson = 3
			expProg = 8
			expLabels = 7
		} else if i == 3 {
			expLesson = 0
			expProg = 11
			expLabels = 7
		} else if i == 4 {
			expLesson = 0
			expProg = 3
			expLabels = 0
		}
		s.EqualValues(expLesson, len(byType[mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_LESSON_PART].ID]), fmt.Sprintf("number of LESSONS, Page n %d", i))
		s.EqualValues(expProg, len(byType[mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_VIDEO_PROGRAM_CHAPTER].ID]), fmt.Sprintf("number of PROGRAMS, Page n %d", i))
		s.EqualValues(expLabels, len(byType[-2]), fmt.Sprintf("number of labels, Page n %d", i))
	}
}

func (s *TagDashboardSuite) TestUniqueItems() {
	//boil.DebugMode = true
	tags := s.mkTags(rand.Intn(10))
	tag := s.mkTags(1)[0]

	lesTags := make([]*mdbmodels.Tag, 0)
	progTags := make([]*mdbmodels.Tag, 0)
	lTags := make([]*mdbmodels.Tag, 0)

	lesTags = append(lesTags, tag)
	progTags = append(progTags, tag)
	lTags = append(lTags, tag)

	for _, t := range tags {
		if x := rand.Intn(100) % 3; x == 1 {
			lesTags = append(lesTags, t)
		} else if x == 2 {
			progTags = append(progTags, t)
		} else {
			lTags = append(lTags, t)
		}
	}

	cusLessons := s.mkCUs(rand.Intn(100), mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_LESSON_PART].ID, lesTags)
	cusPrograms := s.mkCUs(rand.Intn(100), mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_VIDEO_PROGRAM_CHAPTER].ID, progTags)
	labels := s.mkLabels(rand.Intn(100), "media", lTags)

	pageSize := 10
	total := len(cusLessons) + len(cusPrograms) + len(labels)

	lById := make(map[string]bool)
	cuById := make(map[string]bool)
	for i := 1; pageSize*(i-1) <= total; i++ {
		r := api.TagDashboardRequest{
			ListRequest: api.ListRequest{PageNumber: i, PageSize: pageSize, BaseRequest: api.BaseRequest{Language: "he"}},
			TagsFilter:  api.TagsFilter{Tags: []string{tag.UID}},
		}

		resp, _ := api.HandleTagDashboard(common.CACHE, s.DB, r)
		//s.NoError(err)
		count := pageSize
		if i*pageSize > total {
			count = total % pageSize
		}
		if count == 0 {
			count = pageSize
		}
		s.EqualValues(count, len(resp.Items))
		s.EqualValues(total, resp.MediaTotal)
		s.assertUniq(resp.Items, lById, cuById)
	}
}

func (s *TagDashboardSuite) TestUniqueItemsWithLangugaFilter() {
	//boil.DebugMode = true
	tags := s.mkTags(rand.Intn(10))
	tag := s.mkTags(1)[0]

	lesTags := make([]*mdbmodels.Tag, 0)
	progTags := make([]*mdbmodels.Tag, 0)
	lTags := make([]*mdbmodels.Tag, 0)

	lesTags = append(lesTags, tag)
	progTags = append(progTags, tag)
	lTags = append(lTags, tag)

	for _, t := range tags {
		if x := rand.Intn(100) % 3; x == 1 {
			lesTags = append(lesTags, t)
		} else if x == 2 {
			progTags = append(progTags, t)
		} else {
			lTags = append(lTags, t)
		}
	}

	cusLessons := s.mkCUs(rand.Intn(100), mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_LESSON_PART].ID, lesTags)
	cusPrograms := s.mkCUs(rand.Intn(100), mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_VIDEO_PROGRAM_CHAPTER].ID, progTags)
	labels := s.mkLabels(rand.Intn(100), "media", lTags)

	for _, cu := range append(cusLessons, cusPrograms...) {
		s.mkFiles(rand.Intn(10), cu.ID)
	}
	pageSize := 20
	total := len(cusLessons) + len(cusPrograms) + len(labels)

	lById := make(map[string]bool)
	cuById := make(map[string]bool)
	for i := 1; pageSize*(i-1) <= total; i++ {
		r := api.TagDashboardRequest{
			ListRequest: api.ListRequest{PageNumber: i, PageSize: pageSize, BaseRequest: api.BaseRequest{Language: "he"}},
			TagsFilter:  api.TagsFilter{Tags: []string{tag.UID}},
		}

		resp, err := api.HandleTagDashboard(common.CACHE, s.DB, r)
		s.NoError(err)
		count := pageSize
		if i*pageSize > total {
			count = total % pageSize
		}
		s.EqualValues(count, len(resp.Items))
		s.EqualValues(total, resp.MediaTotal)
		s.assertUniq(resp.Items, lById, cuById)
	}
}

//helpers

*/
/*
func (s *TagDashboardSuite) respCusByCT(resp *api.TagsDashboardResponse) map[int64][]string {
	cuByType := make(map[int64][]string, 0)
	// for labels
	cuByType[-2] = make([]string, 0)
	uids := make([]string, len(resp.Items))
	for i, x := range resp.Items {
		if x.LabelID != "" {
			cuByType[-2] = append(cuByType[-2], x.LabelID)
		}
		uids[i] = x.ContentUnitID
	}

	all, err := mdbmodels.ContentUnits(s.DB, qm.WhereIn("uid IN ?", utils.ConvertArgsString(uids)...)).All()
	s.NoError(err)
	for _, unit := range all {
		if _, ok := cuByType[unit.TypeID]; !ok {
			cuByType[unit.TypeID] = make([]string, 0)
		}
		cuByType[unit.TypeID] = append(cuByType[unit.TypeID], unit.UID)
	}
	return cuByType
}

func (s *TagDashboardSuite) mkCUs(n int, typeId int64, tags []*mdbmodels.Tag) []*mdbmodels.ContentUnit {
	cus := make([]*mdbmodels.ContentUnit, n)
	for i, _ := range cus {
		cu := &mdbmodels.ContentUnit{
			UID:       utils.GenerateUID(8),
			Secure:    0,
			TypeID:    typeId,
			Published: true,
		}
		s.NoError(cu.Insert(s.DB))
		s.NoError(cu.AddTags(s.DB, false, tags...))
		i18n := &mdbmodels.ContentUnitI18n{Language: "he", Name: null.StringFrom(fmt.Sprintf("cu %d", cu.ID))}
		s.NoError(cu.AddContentUnitI18ns(s.DB, true, i18n))
		cus[i] = cu
	}
	return cus
}

func (s *TagDashboardSuite) mkLabels(n int, mtype string, tags []*mdbmodels.Tag) []*mdbmodels.Label {
	labels := make([]*mdbmodels.Label, n)
	for i, _ := range labels {
		cu := &mdbmodels.ContentUnit{
			UID:       utils.GenerateUID(8),
			Secure:    0,
			Published: true,
			TypeID:    mdb.CONTENT_TYPE_REGISTRY.ByName[consts.CT_SOURCE].ID,
		}
		s.NoError(cu.Insert(s.DB))

		i18n := &mdbmodels.ContentUnitI18n{Language: "he", Name: null.StringFrom(fmt.Sprintf("label cu %d", cu.ID))}
		s.NoError(cu.AddContentUnitI18ns(s.DB, true, i18n))

		l := &mdbmodels.Label{
			UID:           utils.GenerateUID(8),
			ContentUnitID: cu.ID,
			MediaType:     mtype,
			ApproveState:  consts.APR_APPROVED,
		}

		s.NoError(l.Insert(s.DB))

		lI18n := &mdbmodels.LabelI18n{Language: "he", Name: null.StringFrom("test")}
		s.NoError(l.AddLabelI18ns(s.DB, true, lI18n))
		s.NoError(l.AddTags(s.DB, false, tags...))

		labels[i] = l
	}
	return labels
}

func (s *TagDashboardSuite) mkTags(n int) []*mdbmodels.Tag {
	tags := make([]*mdbmodels.Tag, n)
	for i, _ := range tags {
		tag := &mdbmodels.Tag{
			UID:     utils.GenerateUID(8),
			Pattern: null.StringFrom("test"),
		}
		s.NoError(tag.Insert(s.DB))

		tags[i] = tag
	}

	s.Cache.Refresh()
	return tags
}

func (s *TagDashboardSuite) mkFiles(n int, cuid int64) []*mdbmodels.File {
	files := make([]*mdbmodels.File, n)
	for i, _ := range files {
		lang := consts.ALL_KNOWN_LANGS[rand.Intn(len(consts.ALL_KNOWN_LANGS))]
		f := &mdbmodels.File{
			UID:           utils.GenerateUID(8),
			ContentUnitID: null.Int64From(cuid),
			Language:      null.StringFrom(lang),
			Secure:        0,
			Published:     true,
		}
		s.NoError(f.Insert(s.DB))

		files[i] = f
	}

	s.Cache.Refresh()
	return files
}

func (s *TagDashboardSuite) assertUniq(items []*api.TagsDashboardItem, lById, cuById map[string]bool) {
	for _, item := range items {
		if item.LabelID != "" {
			if _, ok := lById[item.LabelID]; ok {
				s.NoError(errors.New(fmt.Sprintf("label %s is duplicated", item.LabelID)))
				continue
			}
			lById[item.LabelID] = true
			continue
		}
		if _, ok := cuById[item.ContentUnitID]; ok {
			s.NoError(errors.New(fmt.Sprintf("content unit %s is duplicated", item.ContentUnitID)))
			continue
		}
		cuById[item.ContentUnitID] = true
	}
}
*/
