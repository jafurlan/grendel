package frontend

import (
	"fmt"

	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/ubccr/grendel/frontend/views_templ/layouts"
	"github.com/ubccr/grendel/frontend/views_templ/pages"
)

func (h *Handler) Index(f *fiber.Ctx) error {
	return f.Render("index", fiber.Map{
		"Title": "Grendel",
	})
}
func (h *Handler) indexTempl(f *fiber.Ctx) error {
	hosts, err := h.DB.Hosts()
	if err != nil {
		return err
	}

	componentHandler := templ.Handler(layouts.Base(hosts, pages.Index()))
	return adaptor.HTTPHandler(componentHandler)(f)
}

func (h *Handler) Register(f *fiber.Ctx) error {
	return f.Render("register", fiber.Map{
		"Title": "Grendel - Register",
	})
}

func (h *Handler) Login(f *fiber.Ctx) error {
	return f.Render("login", fiber.Map{
		"Title": "Grendel - Login",
	})
}

func (h *Handler) Floorplan(f *fiber.Ctx) error {
	return f.Render("floorplan", fiber.Map{
		"Title": "Grendel - Floorplan",
	})
}

func (h *Handler) Rack(f *fiber.Ctx) error {
	rack := f.Params("rack")
	return f.Render("rack", fiber.Map{
		"Title": fmt.Sprintf("Grendel - %s", rack),
		"Rack":  rack,
	})
}

func (h *Handler) Host(f *fiber.Ctx) error {
	host := f.Params("host")
	return f.Render("host", fiber.Map{
		"Title":    fmt.Sprintf("Grendel - %s", host),
		"HostName": host,
	})
}

func (h *Handler) users(f *fiber.Ctx) error {
	hosts, err := h.DB.Hosts()
	if err != nil {
		return err
	}
	componentHandler := templ.Handler(layouts.Base(hosts, pages.Users()))
	return adaptor.HTTPHandler(componentHandler)(f)
}
