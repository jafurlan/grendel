package frontend

import (
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/segmentio/ksuid"
	"github.com/spf13/viper"
	"github.com/ubccr/grendel/firmware"
	"github.com/ubccr/grendel/model"
	"github.com/ubccr/grendel/nodeset"
)

func (h *Handler) LoginUser(f *fiber.Ctx) error {
	user := f.FormValue("username")
	pass := f.FormValue("password")

	val, role, err := h.DB.VerifyUser(user, pass)
	if err != nil || !val {
		msg := "Internal server error"
		if err.Error() == "crypto/bcrypt: hashedPassword is not the hash of the given password" {
			msg = "Failed to login: Password is incorrect"
		} else if err.Error() == "not found" {
			msg = "Failed to login: Username not found."
		}
		return ToastError(f, err, msg)
	}
	sess, err := h.Store.Get(f)
	if err != nil {
		return ToastError(f, err, "Error getting session")
	}

	sess.Set("authenticated", val)
	sess.Set("user", user)
	sess.Set("role", role)

	if err := sess.Save(); err != nil {
		return ToastError(f, err, "Failed to save session")
	}

	f.Response().Header.Add("HX-Redirect", "/")
	return ToastSuccess(f, "Successfully logged in", ``)
}

func (h *Handler) LogoutUser(f *fiber.Ctx) error {
	sess, _ := h.Store.Get(f)
	sess.Destroy()

	f.Response().Header.Add("HX-Redirect", "/")
	return ToastSuccess(f, "Successfully logged out", ``)
}

func (h *Handler) RegisterUser(f *fiber.Ctx) error {
	u := f.FormValue("username")
	p := f.FormValue("password")

	msg := "Successfully registered user"

	err := h.DB.StoreUser(u, p)
	if err != nil {
		if err.Error() == fmt.Sprintf("user %s already exists", u) {
			msg = "Failed to register: Username already exists"
		} else {
			msg = "Failed to register user"
		}
		return ToastError(f, err, msg)
	}

	return ToastSuccess(f, msg, ``)
}

type FormData struct {
	ID         string `form:"ID"`
	Name       string `form:"Name"`
	Provision  string `form:"Provision"`
	Firmware   string `form:"Firmware"`
	BootImage  string `form:"BootImage"`
	Tags       string `form:"Tags"`
	Interfaces string `form:"Interfaces"`
}

func (h *Handler) EditHost(f *fiber.Ctx) error {
	formHost := new(FormData)

	err := f.BodyParser(formHost)
	if err != nil {
		return ToastError(f, err, "Failed to bind type to request body")
	}

	id, _ := ksuid.Parse(formHost.ID)

	provision := false
	if formHost.Provision == "on" {
		provision = true
	}
	var ifaces []*model.NetInterface
	json.Unmarshal([]byte(formHost.Interfaces), &ifaces)

	newHost := model.Host{
		ID:         id,
		Name:       formHost.Name,
		Provision:  provision,
		Firmware:   firmware.NewFromString(formHost.Firmware),
		BootImage:  formHost.BootImage,
		Tags:       strings.Split(formHost.Tags, ","),
		Interfaces: ifaces,
	}

	err = h.DB.StoreHost(&newHost)
	if err != nil {
		return ToastError(f, err, "Failed to update host")
	}

	return ToastSuccess(f, "Successfully updated host", ``)
}

func (h *Handler) DeleteHost(f *fiber.Ctx) error {
	hosts := f.FormValue("hosts")
	ns, err := nodeset.NewNodeSet(hosts)
	if err != nil {
		return ToastError(f, err, "Failed to parse node set")
	}

	h.DB.DeleteHosts(ns)

	f.Response().Header.Add("HX-Refresh", "true")
	return ToastSuccess(f, "Successfully deleted host(s)", ``)
}

type RebootData struct {
	Host string
}

func (h *Handler) RebootHost(f *fiber.Ctx) error {
	delay := viper.GetInt("bmc.delay")
	fanout := viper.GetInt("bmc.fanout")

	hosts := f.FormValue("hosts")
	ns, err := nodeset.NewNodeSet(hosts)
	if err != nil {
		return ToastError(f, err, "Failed to parse node set")
	}

	hostList, err := h.DB.FindHosts(ns)
	if err != nil {
		return ToastError(f, err, "Failed to find nodes")
	}

	runner := NewJobRunner(fanout)

	for i, host := range hostList {
		runner.RunReboot(host)
		if (i+1)%fanout == 0 {
			time.Sleep(time.Duration(delay) * time.Second)
		}
	}
	runner.Wait()

	// TODO: add channel to get status of each job
	return ToastSuccess(f, "Successfully Rebooted node(s)", ``)
}

func (h *Handler) BmcConfigure(f *fiber.Ctx) error {
	delay := viper.GetInt("bmc.delay")
	fanout := viper.GetInt("bmc.fanout")

	hosts := f.FormValue("hosts")
	ns, err := nodeset.NewNodeSet(hosts)
	if err != nil {
		return ToastError(f, err, "Failed to parse node set")
	}

	hostList, _ := h.DB.FindHosts(ns)
	if err != nil {
		return ToastError(f, err, "Failed to find nodes")
	}

	runner := NewJobRunner(fanout)

	for i, host := range hostList {
		runner.RunConfigure(host)
		if (i+1)%fanout == 0 {
			time.Sleep(time.Duration(delay) * time.Second)
		}
	}
	runner.Wait()

	// TODO: add channel to get status of each job
	return ToastSuccess(f, "Successfully sent Auto Configure node(s)", ``)
}

type hostAddData struct {
	Firmware   string `form:"Firmware"`
	Provision  string `form:"Provision"`
	BootImage  string `form:"BootImage"`
	Tags       string `form:"Tags"`
	MgmtDomain string `form:"Mgmt:Domain"`
	CoreDomain string `form:"Core:Domain"`
	MgmtMtu    string `form:"Mgmt:Mtu"`
	CoreMtu    string `form:"Core:Mtu"`
}

func (h *Handler) HostAdd(f *fiber.Ctx) error {
	var formData hostAddData
	err := f.BodyParser(&formData)
	if err != nil {
		return ToastError(f, err, "Failed to bind form data")
	}

	pString := formData.Provision
	provision := false
	if pString == "on" {
		provision = true
	}

	mgmtMtu, err := strconv.Atoi(formData.MgmtMtu)
	if err != nil {
		return ToastError(f, err, "Failed to parse MTU")
	}
	coreMtu, err := strconv.Atoi(formData.CoreMtu)
	if err != nil {
		return ToastError(f, err, "Failed to parse MTU")
	}

	hostList := strings.Split(f.FormValue("HostList"), ",")

	for _, v := range hostList {
		hostArr := strings.Split(v, "-")
		noPrefix := strings.Join(hostArr[1:], "-")
		bmcHost := fmt.Sprintf("bmc-%s", noPrefix)
		mgmtMac, err := net.ParseMAC(f.FormValue(fmt.Sprintf("%s:MgmtMac", v)))
		if err != nil {
			return ToastError(f, err, "Failed to parse MAC address")
		}
		coreMac, err := net.ParseMAC(f.FormValue(fmt.Sprintf("%s:CoreMac", v)))
		if err != nil {
			return ToastError(f, err, "Failed to parse MAC address")
		}

		mgmtIp, err := netip.ParsePrefix(f.FormValue(fmt.Sprintf("%s:MgmtIp", v)))
		if err != nil {
			return ToastError(f, err, "Failed to parse IP address")
		}
		coreIp, err := netip.ParsePrefix(f.FormValue(fmt.Sprintf("%s:CoreIp", v)))
		if err != nil {
			return ToastError(f, err, "Failed to parse IP address")
		}

		ifaces := make([]*model.NetInterface, 2)
		ifaces[0] = &model.NetInterface{
			Name: f.FormValue("Mgmt:Ifname"),
			FQDN: fmt.Sprintf("%s.%s", bmcHost, formData.MgmtDomain),
			IP:   mgmtIp,
			MAC:  mgmtMac,
			BMC:  true,
			VLAN: f.FormValue("Mgmt:Vlan"),
			MTU:  uint16(mgmtMtu),
		}
		ifaces[1] = &model.NetInterface{
			Name: f.FormValue("Core:Ifname"),
			FQDN: fmt.Sprintf("%s.%s", v, formData.CoreDomain),
			IP:   coreIp,
			MAC:  coreMac,
			BMC:  false,
			VLAN: f.FormValue("Core:Vlan"),
			MTU:  uint16(coreMtu),
		}
		newHost := model.Host{
			ID:         ksuid.New(),
			Name:       v,
			Provision:  provision,
			BootImage:  formData.BootImage,
			Firmware:   firmware.NewFromString(formData.Firmware),
			Tags:       strings.Split(formData.Tags, ","),
			Interfaces: ifaces,
		}
		err = h.DB.StoreHost(&newHost)
		if err != nil {
			return ToastError(f, err, "Failed to add host(s)")
		}

	}

	f.Response().Header.Add("HX-Refresh", "true")
	return ToastSuccess(f, "Successfully added host(s)", ``)
}

func (h *Handler) SwitchMac(f *fiber.Ctx) error {
	rack := f.FormValue("Rack")
	hosts := strings.Split(f.FormValue("HostList"), ",")
	mgmtMacList, err := GetMacAddress(h, f, rack, "Mgmt", hosts)
	if err != nil {
		return ToastError(f, err, "Failed to get mgmt MAC address")
	}
	coreMacList, err := GetMacAddress(h, f, rack, "Core", hosts)
	if err != nil {
		return ToastError(f, err, "Failed to get core MAC address")
	}

	type hostStruct struct {
		Host     string
		MgmtPort string
		CorePort string
		MgmtMac  string
		CoreMac  string
	}
	hostList := make([]hostStruct, len(hosts))

	for i, host := range hosts {
		hostList[i] = hostStruct{
			Host:     host,
			MgmtPort: f.FormValue(fmt.Sprintf("%s:Mgmt", host), ""),
			CorePort: f.FormValue(fmt.Sprintf("%s:Core", host), ""),
			MgmtMac:  mgmtMacList[host],
			CoreMac:  coreMacList[host],
		}
	}

	return f.Render("fragments/hostAddModalList", fiber.Map{
		"Hosts":    hostList,
		"HostList": strings.Join(hosts, ","),
	}, "")
}

func (h *Handler) Search(f *fiber.Ctx) error {
	search := f.FormValue("search")

	f.Response().Header.Add("HX-Redirect", fmt.Sprintf("/host/%s", search))
	return nil
}

func (h *Handler) UsersPost(f *fiber.Ctx) error {
	users := f.FormValue("Usernames")
	role := f.FormValue("Role")
	userList := strings.Split(users, ",")

	for _, user := range userList {
		err := h.DB.UpdateUser(user, role)
		if err != nil {
			return ToastError(f, err, "Failed to update user: "+user)
		}
	}

	return ToastSuccess(f, "Successfully updated user(s)", `, "updateUsers": ""`)
}
