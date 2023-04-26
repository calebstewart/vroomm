package virt

import (
	"libvirt.org/go/libvirt"
)

type Connection struct {
	*libvirt.Connect // The underlying libvirt connection
}

func New(connectionUri string, handler *libvirt.ConnectAuth) (*Connection, error) {
	var err error

	conn := &Connection{}

	conn.Connect, err = libvirt.NewConnectWithAuth(connectionUri, handler, 0)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (c *Connection) EnumerateActiveDomains() ([]*Domain, error) {

	rawDomains, err := c.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_ACTIVE)
	if err != nil {
		return nil, err
	}

	domains := []*Domain{}
	for _, rawDomain := range rawDomains {
		if domain, err := NewDomain(rawDomain); err != nil {
			return nil, err
		} else {
			domains = append(domains, domain)
		}
	}

	return domains, nil
}

func (c *Connection) EnumerateAllDomains() ([]*Domain, error) {

	rawDomains, err := c.ListAllDomains(0)
	if err != nil {
		return nil, err
	}

	domains := []*Domain{}
	for _, rawDomain := range rawDomains {
		if domain, err := NewDomain(rawDomain); err != nil {
			return nil, err
		} else {
			domains = append(domains, domain)
		}
	}

	return domains, nil
}

func (c *Connection) EnumerateStoppedDomains() ([]*Domain, error) {
	rawDomains, err := c.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_INACTIVE)
	if err != nil {
		return nil, err
	}

	domains := []*Domain{}
	for _, rawDomain := range rawDomains {
		if domain, err := NewDomain(rawDomain); err != nil {
			return nil, err
		} else {
			domains = append(domains, domain)
		}
	}

	return domains, nil
}
