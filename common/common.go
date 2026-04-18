package common

import (
	"database/sql"
	"time"

	log "github.com/Sirupsen/logrus"
	_ "github.com/lib/pq"
	"github.com/spf13/viper"
	"github.com/volatiletech/sqlboiler/v4/boil"

	"github.com/Bnei-Baruch/archive-backend/api"
	"github.com/Bnei-Baruch/archive-backend/cache"
	"github.com/Bnei-Baruch/archive-backend/consts"
	"github.com/Bnei-Baruch/archive-backend/es"
	"github.com/Bnei-Baruch/archive-backend/integration"
	"github.com/Bnei-Baruch/archive-backend/mdb"
	"github.com/Bnei-Baruch/archive-backend/search"
	llm "github.com/Bnei-Baruch/archive-backend/search/LLM"
	llmtools "github.com/Bnei-Baruch/archive-backend/search/LLM/tools"
	"github.com/Bnei-Baruch/archive-backend/utils"
)

var (
	DB    *sql.DB
	ESC   *search.ESManager
	CACHE cache.CacheManager
	//GRAMMARS     search.Grammars
	VARIABLES    search.VariablesV2
	TOKENS_CACHE *search.TokensCache
	CMS          *api.CMSParams
	ASSETS       integration.AssetsService
	LLM_RUNTIME  *llm.Runtime
)

func Init() time.Time {
	return InitWithDefault(nil, nil)
}

func InitWithDefault(defaultDb *sql.DB, defaultCache *cache.CacheManager) time.Time {
	var err error
	clock := time.Now()

	CMS = &api.CMSParams{
		Assets: viper.GetString("cms.assets"),
		Mode:   viper.GetString("server.mode"),
	}

	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	//log.SetLevel(log.WarnLevel)

	if defaultDb != nil {
		DB = defaultDb
	} else {
		log.Info("Setting up connection to MDB")
		DB, err = sql.Open("postgres", viper.GetString("mdb.url"))
		utils.Must(err)
		utils.Must(DB.Ping())

		if val := viper.GetInt("mdb.max-idle-conns"); val > 0 {
			DB.SetMaxIdleConns(val)
		}
		if val := viper.GetInt("mdb.max-open-conns"); val > 0 {
			DB.SetMaxOpenConns(val)
		}
		if val := viper.GetDuration("mdb.conn-max-idle-time"); val > 0 {
			DB.SetConnMaxIdleTime(val)
		}
		if val := viper.GetDuration("mdb.conn-max-lifetime"); val > 0 {
			DB.SetConnMaxLifetime(val)
		}
	}
	boil.SetDB(DB)
	boil.DebugMode = viper.GetString("server.boiler-mode") == "debug"
	log.Info("Initializing type registries")
	utils.Must(mdb.InitTypeRegistries(DB))

	log.Info("Setting up connection to ElasticSearch")
	url := viper.GetString("elasticsearch.url")
	ESC = search.MakeESManager(url)

	esc, err := ESC.GetClient()
	if esc != nil && err == nil {
		esversion, err := esc.ElasticsearchVersion(url)
		utils.Must(err)
		log.Infof("Elasticsearch version %s", esversion)
	}

	es.InitEnv()

	TOKENS_CACHE = search.MakeTokensCache(consts.TOKEN_CACHE_SIZE)

	// Moving to Grammars V2 that are indexed and searched.
	VARIABLES, err = search.MakeVariablesV2(es.DataFolder("search", "variables"))
	//utils.Must(err)
	//GRAMMARS, err = search.MakeGrammars(viper.GetString("elasticsearch.grammars"), esc, TOKENS_CACHE, VARIABLES)
	//utils.Must(err)
	if defaultCache == nil {
		viper.SetDefault("cache.refresh-search-stats", 5*time.Minute)
		refreshIntervals := map[string]time.Duration{
			"SearchStats": viper.GetDuration("cache.refresh-search-stats"),
		}
		CACHE = cache.NewCacheManagerImpl(DB, refreshIntervals)
	} else {
		CACHE = *defaultCache
	}

	ASSETS = integration.NewAssetsService(viper.GetString("assets_service.url"))
	//	Progress tracks transient UI/status updates for the current run
	progress := llm.NewReasoningProgressStore(llm.ReasoningSessionTTLFromConfig())
	//	Workflow stores stage/provider/session metadata needed to resume or rerun correctly
	workflow := llm.NewReasoningWorkflowSessionStore(llm.ReasoningSessionTTLFromConfig())
	var reasoningCache *llm.ReasoningSearchCacheStore
	if llm.ReasoningSearchCacheEnabledFromConfig() {
		reasoningCache = llm.NewReasoningSearchCacheStore(llm.ReasoningSearchCacheTTLFromConfig())
	}
	defaultProvider := llm.ProviderFromConfig()
	planningProvider := llm.ReasoningSearchPlanningProviderFromConfig()
	verificationProvider := llm.ReasoningSearchVerificationProviderFromConfig()
	aiToolsConfig, err := llm.AIToolsConfigFromConfig()
	utils.Must(err)
	services := map[string]llm.Service{}

	services[defaultProvider], err = llm.NewServiceForProviderWithProgress(defaultProvider, progress)
	utils.Must(err)
	if viper.GetBool("llm.reasoning-search-verification-enabled") {
		if _, ok := services[verificationProvider]; !ok {
			services[verificationProvider], err = llm.NewServiceForProviderWithProgress(verificationProvider, nil)
			utils.Must(err)
		}
	}
	if viper.GetBool("llm.reasoning-search-planning-enabled") {
		if _, ok := services[planningProvider]; !ok {
			services[planningProvider], err = llm.NewServiceForProviderWithProgress(planningProvider, nil)
			utils.Must(err)
		}
	}
	if _, ok := services[aiToolsConfig.Provider]; !ok {
		services[aiToolsConfig.Provider], err = llm.NewServiceForProviderWithProgress(aiToolsConfig.Provider, nil)
		utils.Must(err)
	}

	var postgreSQLToolCacheTTL *time.Duration
	if viper.IsSet("llm.postgresql-tool-cache-ttl") {
		ttl := viper.GetDuration("llm.postgresql-tool-cache-ttl")
		postgreSQLToolCacheTTL = &ttl
	}
	tools, err := llmtools.NewAppScopedManager(llmtools.AppScopedManagerDeps{
		DB:             DB,
		AssetsService:  ASSETS,
		AIQueryService: services[aiToolsConfig.Provider],
		AIQueryConfig:  aiToolsConfig,
		NewElasticsearchSearchEngine: func() (llmtools.ElasticsearchSearchEngine, error) {
			esc, err := ESC.GetClient()
			if err != nil {
				return nil, err
			}
			return search.NewESEngine(esc, DB, CACHE, TOKENS_CACHE, VARIABLES, consts.ES_SEARCH_RESULT_TYPES), nil
		},
		TimeoutForHighlight:    viper.GetDuration("elasticsearch.timeout-for-highlight"),
		PostgreSQLToolCacheTTL: postgreSQLToolCacheTTL,
	})
	utils.Must(err)

	LLM_RUNTIME = &llm.Runtime{
		Tools:          tools,
		Progress:       progress,
		Workflow:       workflow,
		ReasoningCache: reasoningCache,
		Services:       services,
	}

	return clock
}

func Shutdown() {
	if LLM_RUNTIME != nil {
		closed := map[llm.Service]bool{}
		for _, service := range LLM_RUNTIME.Services {
			if service == nil || closed[service] {
				continue
			}
			if closer, ok := service.(interface{ Close() error }); ok {
				utils.Must(closer.Close())
			}
			closed[service] = true
		}
		if LLM_RUNTIME.Progress != nil {
			utils.Must(LLM_RUNTIME.Progress.Close())
		}
		if LLM_RUNTIME.Workflow != nil {
			utils.Must(LLM_RUNTIME.Workflow.Close())
		}
		if LLM_RUNTIME.ReasoningCache != nil {
			utils.Must(LLM_RUNTIME.ReasoningCache.Close())
		}
	}
	utils.Must(DB.Close())
	ESC.Stop()
	CACHE.Close()
}
