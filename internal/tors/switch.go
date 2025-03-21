// SPDX-FileCopyrightText: (C) 2019 Grendel Authors
//
// SPDX-License-Identifier: GPL-3.0-or-later

package tors

import (
	"encoding/json"
	"errors"
	"net"

	"github.com/spf13/viper"
	"github.com/ubccr/grendel/internal/logger"
	"github.com/ubccr/grendel/pkg/model"
)

var log = logger.GetLogger("SWITCH")

type InterfaceStatus struct {
	Name                string           // `json:"name"`
	LineProtocolStatus  string           // `json:"lineProtocolStatus"`
	InterfaceStatus     string           // `json:"interfaceStatus"`
	PhysicalAddress     net.HardwareAddr // `json:"physicalAddress"`
	Description         string           // `json:"description"`
	Hardware            string           // `json:"hardware"`
	Bandwidth           int              // `json:"bandwidth"`
	MTU                 int              // `json:"mtu"`
	InterfaceMembership string           // `json:"interfaceMembership"`
	// Duplex              string           // `json:"duplex"`
	// AutoNegotiate       string           // `json:"autoNegotiate"`
	// Lanes               string           // `json:"lanes"`
}

type MACTableEntry struct {
	Ifname string           `json:"ifname"`
	Port   int              `json:"port"`
	VLAN   string           `json:"vlan"`
	Type   string           `json:"type"`
	MAC    net.HardwareAddr `json:"mac-addr"`
}

type LLDP struct {
	ChassisIdType     string
	ChassisId         net.HardwareAddr
	SystemName        string
	SystemDescription string
	ManagementAddress string
	PortDescription   string
	PortId            string
	PortIdType        string
}

type InterfaceTable map[int]*InterfaceStatus

type MACTable map[string]*MACTableEntry

type LLDPNeighbors map[string]*LLDP

type NetworkSwitch interface {
	GetInterfaceStatus() (InterfaceTable, error)
	GetMACTable() (MACTable, error)
	GetLLDPNeighbors() (LLDPNeighbors, error)
}

func NewNetworkSwitch(host *model.Host) (NetworkSwitch, error) {
	username := viper.GetString("bmc.switch_admin_username")
	password := viper.GetString("bmc.switch_admin_password")

	if username == "" || password == "" {
		log.Warn("Please set both bmc.switch_admin_username and bmc.switch_admin_password in your toml configuration file in order to query network switches")
		return nil, errors.New("failed to get switch credentials from config file")
	}

	var sw NetworkSwitch
	var err error

	bmc := host.InterfaceBMC()
	ip := ""
	if bmc != nil {
		ip = bmc.AddrString()
	}
	// TODO: automatically determine NOS
	if host.HasTags("arista") {
		sw, err = NewArista(ip, username, password)
	} else if host.HasTags("sonic") {
		sw, err = NewSonic(ip, username, password, "", true)
	} else if host.HasTags("os10") {
		sw, err = NewDellOS10("https://"+ip, username, password, "", true)
	} else {
		return nil, errors.New("failed to determine switch NOS")
	}

	return sw, err
}

func (mt MACTable) Port(port int) []*MACTableEntry {
	entries := make([]*MACTableEntry, 0)
	for _, entry := range mt {
		if entry.Port == port {
			entries = append(entries, entry)
		}
	}

	return entries
}

func (m *MACTableEntry) MarshalJSON() ([]byte, error) {
	type Alias MACTableEntry
	return json.Marshal(&struct {
		MAC string `json:"mac-addr"`
		*Alias
	}{
		MAC:   m.MAC.String(),
		Alias: (*Alias)(m),
	})
}

func (mt MACTable) String() string {
	data, _ := json.MarshalIndent(mt, "", "    ")
	return string(data)
}
