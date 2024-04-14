package frontend

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/favicon"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/spf13/viper"
	"github.com/ubccr/grendel/frontend/types"
	"github.com/ubccr/grendel/model"
)

//go:embed public
var embedFS embed.FS

type Handler struct {
	DB     model.DataStore
	Store  *session.Store
	Events []types.EventStruct
}

func NewHandler(db model.DataStore, Store *session.Store) (*Handler, error) {
	h := &Handler{
		DB:     db,
		Store:  Store,
		Events: []types.EventStruct{},
	}

	return h, nil
}

func (h *Handler) SetupRoutes(app *fiber.App) {
	fragment := app.Group("/fragments")
	api := app.Group("/api")

	auth := h.EnforceAuthMiddleware()
	admin := h.EnforceAdminMiddleware()
	app.Use(func(c *fiber.Ctx) error {
		hostList, err := h.DB.Hosts()
		if err != nil {
			log.Error(err)
		}

		sess, _ := h.Store.Get(c)
		a := sess.Get("authenticated")
		u := sess.Get("user")
		r := sess.Get("role")
		c.Context().SetUserValue("path", c.Path())
		c.Context().SetUserValue("authenticated", a)
		c.Context().SetUserValue("user", u)
		c.Context().SetUserValue("role", r)
		c.Context().SetUserValue("events", h.Events)

		err = c.Bind(fiber.Map{
			"Auth": fiber.Map{
				"Authenticated": a,
				"User":          u,
				"Role":          r,
			},
			"SearchList":  hostList,
			"CurrentPath": c.Path(),
			"Events":      h.Events,
		})
		if sess.Get("role") == "disabled" {
			c.Response().Header.Add("HX-Trigger", `{"toast-error": "Your account is disabled. Please ask an Administrator to activate your account."}`)
		}
		if err != nil {
			log.Error(err)
		}
		return c.Next()
	})

	public, err := fs.Sub(embedFS, "public")
	if err != nil {
		log.Error("failed to load public files")
	}

	app.Use("/static", filesystem.New(filesystem.Config{
		Root:   http.FS(public),
		Browse: false,
	}))

	if viper.IsSet("frontend.favicon") {
		app.Use(favicon.New(favicon.Config{
			File: viper.GetString("frontend.favicon"),
		}))
	} else {
		app.Use(favicon.New(favicon.Config{
			File:       "favicon.ico",
			FileSystem: http.FS(public),
		}))
	}

	app.Get("/", h.Index)
	app.Get("/templ", h.indexTempl)

	app.Get("/login", h.Login)
	api.Post("/auth/login", h.LoginUser)
	api.Post("/auth/logout", h.LogoutUser)

	app.Get("/register", h.Register)
	api.Post("/auth/register", h.RegisterUser)

	app.Get("/host/:host", auth, h.Host)
	fragment.Get("/host/:host/form", auth, h.hostForm)
	api.Post("/host", auth, h.EditHost)
	api.Delete("/host", auth, h.DeleteHost)
	api.Post("/host/import", auth, h.importHost)

	fragment.Get("/interfaces", auth, h.interfaces)

	app.Get("/floorplan", auth, h.Floorplan)
	fragment.Get("/floorplan/table", auth, h.floorplanTable)
	fragment.Get("/floorplan/modal", auth, h.floorplanModal)

	app.Get("/rack/:rack", auth, h.Rack)
	fragment.Get("/rack/:rack/table", auth, h.rackTable)
	fragment.Get("/rack/:rack/actions", auth, h.rackActions)
	fragment.Get("/rack/:rack/add/modal", auth, h.rackAddModal)
	fragment.Post("/rack/:rack/add/table", auth, h.rackAddTable)

	api.Post("/bulkHostAdd", auth, h.bulkHostAdd)

	api.Patch("/hosts/provision", auth, h.provisionHosts)
	api.Patch("/hosts/tags", auth, h.tagHosts)
	api.Patch("/hosts/image", auth, h.imageHosts)
	api.Get("/hosts/export/:hosts", auth, h.exportHosts)

	app.Get("/users", admin, h.users)
	app.Get("/templ/users", admin, h.users)
	api.Post("/users", admin, h.usersPost)
	fragment.Get("/users/table", admin, h.usersTable)
	api.Delete("/user/:username", admin, h.deleteUser)

	api.Get("/search", auth, h.Search)
	api.Get("/search/list", auth, h.searchList)

	fragment.Get("/events", auth, h.events)

	api.Post("/bmc/powerCycle", auth, h.bmcPowerCycle)
	api.Post("/bmc/powerCycleBmc", auth, h.bmcPowerCycleBmc)
	api.Post("/bmc/clearSel", auth, h.bmcClearSel)
	api.Post("/bmc/configure/auto", auth, h.bmcConfigureAuto)
	api.Post("/bmc/configure/import", auth, h.bmcConfigureImport)
}
