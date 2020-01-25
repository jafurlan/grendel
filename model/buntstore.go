package model

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/segmentio/ksuid"
	"github.com/sirupsen/logrus"
	"github.com/tidwall/buntdb"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"github.com/ubccr/grendel/nodeset"
)

const (
	HostKeyPrefix      = "host"
	BootImageKeyPrefix = "host"
)

// BuntStore implements a Grendel Datastore using BuntDB
type BuntStore struct {
	db *buntdb.DB
}

// NewBuntStore returns a new BuntStore using the given database filename. For memory only you can provide `:memory:`
func NewBuntStore(filename string) (*BuntStore, error) {
	db, err := buntdb.Open(filename)
	if err != nil {
		return nil, err
	}

	err = db.CreateIndex("id", HostKeyPrefix+":*", buntdb.IndexJSON("id"))
	if err != nil && err != buntdb.ErrIndexExists {
		return nil, err
	}

	return &BuntStore{db: db}, nil
}

// Close closes the BuntStore database
func (s *BuntStore) Close() error {
	return s.db.Close()
}

// StoreHost stores a host in the data store. If the host exists it is overwritten
func (s *BuntStore) StoreHost(host *Host) error {
	hostList := HostList{host}
	return s.StoreHosts(hostList)
}

// StoreHosts stores a list of host in the data store. If the host exists it is overwritten
func (s *BuntStore) StoreHosts(hosts HostList) error {
	for _, host := range hosts {
		if host.Name == "" {
			return fmt.Errorf("name required:  %w", ErrInvalidData)
		}

		// Keys are case-insensitive
		host.Name = strings.ToLower(host.Name)

		// XXX need to check for dups?
		if host.ID.IsNil() {
			uuid, err := ksuid.NewRandom()
			if err != nil {
				return err
			}

			host.ID = uuid
		}
	}

	err := s.db.Update(func(tx *buntdb.Tx) error {
		for _, host := range hosts {
			val, err := json.Marshal(host)
			if err != nil {
				return err
			}

			hostJSON := string(val)

			_, _, err = tx.Set(HostKeyPrefix+":"+host.Name, hostJSON, nil)
			return err
		}
		return nil
	})

	if err != nil {
		return err
	}

	return err
}

// LoadHostFromName returns the Host with the given name
func (s *BuntStore) LoadHostFromName(name string) (*Host, error) {
	var host *Host

	err := s.db.View(func(tx *buntdb.Tx) error {
		val, err := tx.Get(HostKeyPrefix+":"+name, false)
		if err != nil {
			if err != buntdb.ErrNotFound {
				return err
			}

			return nil
		}

		host = &Host{}
		host.FromJSON(val)

		return nil
	})

	if err != nil {
		return nil, err
	}

	if host == nil {
		return nil, fmt.Errorf("host with name %s:  %w", name, ErrNotFound)
	}

	return host, nil
}

// LoadHostFromID returns the Host with the given ID
func (s *BuntStore) LoadHostFromID(id string) (*Host, error) {
	hostJSON := ""

	err := s.db.View(func(tx *buntdb.Tx) error {
		err := tx.AscendEqual("id", fmt.Sprintf(`{"id":"%s"}`, id), func(key, value string) bool {
			hostJSON = value

			// XXX What to about dups? We only fetch first one.
			return false
		})

		return err
	})

	if err != nil {
		return nil, err
	}

	if hostJSON == "" {
		return nil, fmt.Errorf("host with id %s:  %w", id, ErrNotFound)
	}

	host := &Host{}
	host.FromJSON(hostJSON)
	return host, nil
}

// ResolveFQDN returns the list of IPs with the given FQDN
func (s *BuntStore) ResolveFQDN(fqdn string) ([]net.IP, error) {
	ips := make([]net.IP, 0)

	err := s.db.View(func(tx *buntdb.Tx) error {
		err := tx.AscendKeys(HostKeyPrefix+":*", func(key, value string) bool {
			res := gjson.Get(value, "interfaces")
			for _, i := range res.Array() {
				if i.Get("fqdn").String() == fqdn {
					ip := net.ParseIP(i.Get("ip").String())
					if ip != nil && ip.To4() != nil {
						ips = append(ips, ip)
					}

					// XXX stop after first match. consider changing this
					return false
				}
			}

			return true
		})

		return err
	})

	if err != nil {
		return nil, err
	}

	return ips, nil
}

// ReverseResolve returns the list of FQDNs for the given IP
func (s *BuntStore) ReverseResolve(ip string) ([]string, error) {
	fqdn := make([]string, 0)

	err := s.db.View(func(tx *buntdb.Tx) error {
		err := tx.AscendKeys(HostKeyPrefix+":*", func(key, value string) bool {
			res := gjson.Get(value, "interfaces")
			for _, i := range res.Array() {
				if i.Get("ip").String() == ip {
					name := i.Get("fqdn").String()
					if name != "" {
						fqdn = append(fqdn, name)
					}

					// XXX stop after first match. consider changing this
					return false
				}
			}

			return true
		})

		return err
	})

	if err != nil {
		return nil, err
	}

	return fqdn, nil
}

// LoadHostFromMAC returns the Host that has a network interface with the give MAC address
func (s *BuntStore) LoadHostFromMAC(mac string) (*Host, error) {
	hostJSON := ""

	err := s.db.View(func(tx *buntdb.Tx) error {
		err := tx.AscendKeys(HostKeyPrefix+":*", func(key, value string) bool {
			res := gjson.Get(value, "interfaces")
			for _, i := range res.Array() {
				if i.Get("mac").String() == mac {
					hostJSON = value
					return false
				}
			}

			return true
		})

		return err
	})

	if err != nil {
		return nil, err
	}

	if hostJSON == "" {
		return nil, fmt.Errorf("no host found with mac address %s:  %w", mac, ErrNotFound)
	}

	host := &Host{}
	host.FromJSON(hostJSON)
	return host, nil
}

// Hosts returns a list of all the hosts
func (s *BuntStore) Hosts() (HostList, error) {
	hosts := make(HostList, 0)

	err := s.db.View(func(tx *buntdb.Tx) error {
		err := tx.AscendKeys(HostKeyPrefix+":*", func(key, value string) bool {
			h := &Host{}
			h.FromJSON(value)
			hosts = append(hosts, h)
			return true
		})

		return err
	})

	if err != nil {
		return nil, err
	}

	return hosts, nil
}

// FindHosts returns a list of all the hosts in the given NodeSet
func (s *BuntStore) FindHosts(ns *nodeset.NodeSet) (HostList, error) {
	hosts := make(HostList, 0)

	it := ns.Iterator()

	err := s.db.View(func(tx *buntdb.Tx) error {
		for it.Next() {
			val, err := tx.Get(HostKeyPrefix+":"+it.Value(), false)
			if err != nil {
				if err != buntdb.ErrNotFound {
					return err
				}
				continue
			}

			h := &Host{}
			h.FromJSON(val)
			hosts = append(hosts, h)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return hosts, nil
}

// ProvisionHosts sets all hosts in the given NodeSet to provision (true) or unprovision (false)
func (s *BuntStore) ProvisionHosts(ns *nodeset.NodeSet, provision bool) error {
	it := ns.Iterator()

	err := s.db.Update(func(tx *buntdb.Tx) error {
		for it.Next() {
			key := HostKeyPrefix + ":" + it.Value()
			val, err := tx.Get(key, false)
			if err != nil {
				if err != buntdb.ErrNotFound {
					return err
				}
				continue
			}

			val, err = sjson.Set(val, "provision", provision)
			if err != nil {
				return err
			}

			_, _, err = tx.Set(key, val, nil)
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

// SetBootImage sets all hosts to use the BootImage with the given name
func (s *BuntStore) SetBootImage(ns *nodeset.NodeSet, name string) error {
	it := ns.Iterator()

	err := s.db.Update(func(tx *buntdb.Tx) error {
		for it.Next() {
			key := HostKeyPrefix + ":" + it.Value()
			val, err := tx.Get(key, false)
			if err != nil {
				if err != buntdb.ErrNotFound {
					return err
				}
				continue
			}

			val, err = sjson.Set(val, "boot_image", name)
			if err != nil {
				return err
			}

			_, _, err = tx.Set(key, val, nil)
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

// StoreBootImage stores the BootImage in the data store
func (s *BuntStore) StoreBootImage(image *BootImage) error {
	if image.Name == "" {
		return fmt.Errorf("name required:  %w", ErrInvalidData)
	}

	// Keys are case-insensitive
	image.Name = strings.ToLower(image.Name)

	// XXX need to check for dups?
	if image.ID.IsNil() {
		uuid, err := ksuid.NewRandom()
		if err != nil {
			return err
		}

		image.ID = uuid
	}

	val, err := json.Marshal(image)
	if err != nil {
		return err
	}

	imageJSON := string(val)

	err = s.db.Update(func(tx *buntdb.Tx) error {
		_, _, err := tx.Set(BootImageKeyPrefix+":"+image.Name, imageJSON, nil)
		return err
	})

	if err != nil {
		return err
	}

	return err
}

// LoadBootImage returns a BootImage with the given name
func (s *BuntStore) LoadBootImage(name string) (*BootImage, error) {
	var image *BootImage

	err := s.db.View(func(tx *buntdb.Tx) error {
		val, err := tx.Get(BootImageKeyPrefix+":"+name, false)
		if err != nil {
			if err != buntdb.ErrNotFound {
				return err
			}

			return nil
		}

		var i BootImage
		err = json.Unmarshal([]byte(val), &i)
		if err != nil {
			return err
		}

		image = &i
		return nil
	})

	if err != nil {
		return nil, err
	}

	if image == nil {
		return nil, fmt.Errorf("boot image with name %s:  %w", name, ErrNotFound)
	}

	return image, nil
}

// BootImages returns a list of all boot images
func (s *BuntStore) BootImages() (BootImageList, error) {
	images := make(BootImageList, 0)

	err := s.db.View(func(tx *buntdb.Tx) error {
		err := tx.AscendKeys(BootImageKeyPrefix+":*", func(key, value string) bool {
			var i BootImage
			err := json.Unmarshal([]byte(value), &i)
			if err == nil {
				images = append(images, &i)
			} else {
				log.WithFields(logrus.Fields{
					"err": err,
				}).Warn("Invalid boot image json stored in db")
			}
			return true
		})

		return err
	})

	if err != nil {
		return nil, err
	}

	return images, nil
}