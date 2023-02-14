// (C) Copyright Confidential Containers Contributors
// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"context"
	"net/url"
	"sync"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/session/keepalive"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/soap"
)

var glock sync.Mutex

func NewGovmomiClient(vmcfg Config) (*govmomi.Client, error) {

	ctx := context.TODO()

	urlinfo, err := soap.ParseURL(vmcfg.VcenterURL)
	if err != nil {
		return nil, err
	}

	urlinfo.User = url.UserPassword(vmcfg.UserName, vmcfg.Password)

	insecure := vmcfg.Thumbprint == ""
	soapClient := soap.NewClient(urlinfo, insecure)
	if !insecure {
		soapClient.SetThumbprint(urlinfo.Host, vmcfg.Thumbprint)
	}

	vim25Client, err := vim25.NewClient(ctx, soapClient)
	if err != nil {
		logger.Printf("Error creating vcenter session for user %s: error %s", vmcfg.UserName, err)
		return nil, err
	}

	vim25Client.RoundTripper = keepalive.NewHandlerSOAP(vim25Client.RoundTripper, 1*time.Minute, soapKeepAliveHandler(ctx, vim25Client))

	manager := session.NewManager(vim25Client)
	err = manager.Login(ctx, urlinfo.User)
	if err != nil {
		logger.Printf("Error logging in user %s: error %s", vmcfg.UserName, err)
		return nil, err
	}

	logger.Printf("Created session for user %s", vmcfg.UserName)

	gclient := govmomi.Client{
		Client:         vim25Client,
		SessionManager: manager,
	}

	return &gclient, nil
}

func soapKeepAliveHandler(ctx context.Context, c *vim25.Client) func() error {

	return func() error {

		_, err := methods.GetCurrentTime(ctx, c)
		if err != nil {
			logger.Printf("SOAP keep-alive handler error %s", err)
			return err
		}

		return nil
	}
}

func DeleteGovmomiClient(gclient *govmomi.Client) error {

	glock.Lock()

	defer glock.Unlock()

	err := gclient.SessionManager.Logout(context.Background())
	if err != nil {
		logger.Printf("Vcenter logout failed error: %s", err)
	}

	return err
}

func CheckSessionWithRestore(ctx context.Context, vmcfg *Config, gclient *govmomi.Client) error {

	glock.Lock()

	defer glock.Unlock()

	active, err := gclient.SessionManager.SessionIsActive(ctx)

	if active {
		if err == nil {
			return nil
		}
	}

	if err != nil {
		logger.Printf("Creating new sesssion for user %s due to current session error: %s", vmcfg.UserName, err)
	}

	_ = gclient.SessionManager.Logout(ctx) // Cleanup purposes

	urlinfo, _ := soap.ParseURL(vmcfg.VcenterURL)

	urlinfo.User = url.UserPassword(vmcfg.UserName, vmcfg.Password)

	err = gclient.SessionManager.Login(ctx, urlinfo.User)
	if err != nil {
		logger.Printf("Vcenter login failed error: %s", err)
		return err
	}

	logger.Printf("Created new session for user %s", vmcfg.UserName)

	return nil
}
