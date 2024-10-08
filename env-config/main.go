package env_config

import (
	"os"
	"strconv"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/ghupdate"
	"github.com/pocketbase/pocketbase/plugins/jsvm"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
)

func Init(app *pocketbase.PocketBase, collections ...string) error {

	var hooksDir string = os.Getenv("PB_HOOKS_DIR")
	var hooksWatch bool = true
	var hooksPool int = 25
	var migrationsDir string
	var automigrate bool = true
	var publicDir string = "./pb_public"
	var indexFallback bool = true
	var queryTimeout int = 30

	if os.Getenv("PB_HOOKS_DIR") != "" {
		hooksDir = os.Getenv("PB_HOOKS_DIR")
	}

	if os.Getenv("PB_HOOKS_WATCH") != "" {
		value, err := strconv.ParseBool(os.Getenv("PB_HOOKS_WATCH"))
		if err != nil {
			hooksWatch = value
		}
	}

	if os.Getenv("PB_HOOKS_POOL") != "" {
		value, err := strconv.Atoi(os.Getenv("PB_HOOKS_POOL"))
		if err == nil {
			hooksPool = value
		}
	}

	if os.Getenv("PB_AUTO_MIGRATE") != "" {
		value, err := strconv.ParseBool(os.Getenv("PB_AUTO_MIGRATE"))
		if err != nil {
			automigrate = value
		}
	}

	if os.Getenv("PB_PUBLIC_DIR") != "" {
		publicDir = os.Getenv("PB_PUBLIC_DIR")
	}

	if os.Getenv("PB_INDEX_FALLBACK") != "" {
		value, err := strconv.ParseBool(os.Getenv("PB_INDEX_FALLBACK"))
		if err != nil {
			indexFallback = value
		}
	}

	if os.Getenv("PB_QUERY_TIMEOUT") != "" {
		value, err := strconv.Atoi(os.Getenv("PB_QUERY_TIMEOUT"))
		if err == nil {
			queryTimeout = value
		}
	}

	jsvm.MustRegister(app, jsvm.Config{
		MigrationsDir: migrationsDir,
		HooksDir:      hooksDir,
		HooksWatch:    hooksWatch,
		HooksPoolSize: hooksPool,
	})

	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		TemplateLang: migratecmd.TemplateLangJS,
		Automigrate:  automigrate,
		Dir:          migrationsDir,
	})

	// Owner specifies the account owner of the repository (default to "pocketbase").
	Owner := "pocketbase"
	if os.Getenv("PB_GH_UPDATE_OWNER") != "" {
		Owner = os.Getenv("PB_GH_UPDATE_OWNER")
	}

	// Repo specifies the name of the repository (default to "pocketbase").
	Repo := "pocketbase"
	if os.Getenv("PB_GH_UPDATE_REPO") != "" {
		Repo = os.Getenv("PB_GH_UPDATE_REPO")
	}

	// ArchiveExecutable specifies the name of the executable file in the release archive
	// (default to "pocketbase"; an additional ".exe" check is also performed as a fallback).
	ArchiveExecutable := "pocketbase"
	if os.Getenv("PB_GH_UPDATE_ARCHIVE_EXECUTABLE") != "" {
		ArchiveExecutable = os.Getenv("PB_GH_UPDATE_ARCHIVE_EXECUTABLE")
	}

	ghupdate.MustRegister(app, app.RootCmd, ghupdate.Config{
		Owner:             Owner,
		Repo:              Repo,
		ArchiveExecutable: ArchiveExecutable,
	})

	app.OnAfterBootstrap().PreAdd(func(e *core.BootstrapEvent) error {
		app.Dao().ModelQueryTimeout = time.Duration(queryTimeout) * time.Second

		return nil
	})

	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		e.Router.GET("/*", apis.StaticDirectoryHandler(os.DirFS(publicDir), indexFallback), crossOriginHeaders)
		app.OnFileDownloadRequest().Add(func(e *core.FileDownloadEvent) error {
			e.HttpContext.Response().Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			return nil
		})
		return nil
	})

	return nil
}

func crossOriginHeaders(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		c.Response().Header().Set("Cross-Origin-Embedder-Policy", "credentialless")
		if err := next(c); err != nil {
			c.Error(err)
		}
		return nil
	}
}

// func defaultCacheHeaders(next echo.HandlerFunc) echo.HandlerFunc {
// 	return func(c echo.Context) error {
// 		// c.Response().Header().Set("Cache-Control", "public, max-age=3600, stale-while-revalidate=86400")
// 		c.Response().Header().Set("Cache-Control", "public, max-age=1, stale-while-revalidate=59")
// 		if err := next(c); err != nil {
// 			c.Error(err)
// 		}
// 		return nil
// 	}
// }
