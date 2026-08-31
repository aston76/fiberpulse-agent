package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"fiberpulse.dev/agent/internal/accountsync"
)

func (a *App) accountStatus(ctx context.Context) AccountStatus {
	status := AccountStatus{Available: a.accountClient != nil && a.config.OpenURL != nil}
	var saved persistedAccountConnection
	if found, err := a.store.GetSetting(ctx, accountConnectionSetting, &saved); err == nil && found {
		status.Connected = saved.AccessToken != ""
		status.Email = saved.Email
		status.Plan = saved.Plan
		status.SubscriptionStatus = saved.SubscriptionStatus
		status.GoogleLinked = saved.GoogleLinked
		status.LastSyncAt = saved.LastSyncAt
		status.LastError = saved.LastError
	}
	a.accountMu.Lock()
	status.Pairing = a.accountPairing
	a.accountMu.Unlock()
	return status
}

func (a *App) startAccountConnection(ctx context.Context) error {
	if a.accountClient == nil || a.config.OpenURL == nil {
		return errors.New("account connection is unavailable")
	}
	a.accountMu.Lock()
	if a.accountPairing {
		a.accountMu.Unlock()
		return errors.New("an account connection is already in progress")
	}
	a.accountPairing = true
	a.accountMu.Unlock()

	deviceName, err := os.Hostname()
	if err != nil || deviceName == "" {
		deviceName = "FiberPulse desktop"
	}
	request, err := a.accountClient.Start(ctx, deviceName)
	if err != nil {
		a.finishAccountPairing()
		return fmt.Errorf("start account connection: %w", err)
	}
	if err := a.config.OpenURL(request.VerificationURL); err != nil {
		a.finishAccountPairing()
		return fmt.Errorf("open account connection: %w", err)
	}
	a.runAsync(func() { a.waitForAccountAuthorization(request) })
	return nil
}

func (a *App) waitForAccountAuthorization(request accountsync.DeviceRequest) {
	defer a.finishAccountPairing()
	interval := time.Duration(request.Interval) * time.Second
	if interval < 2*time.Second {
		interval = 2 * time.Second
	}
	expires := time.NewTimer(time.Duration(request.ExpiresIn) * time.Second)
	defer expires.Stop()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-expires.C:
			a.saveAccountError("The account connection request expired. Try again.")
			return
		case <-ticker.C:
			exchange, err := a.accountClient.Exchange(a.ctx, request.DeviceCode)
			if errors.Is(err, accountsync.ErrAuthorizationPending) {
				continue
			}
			if err != nil {
				a.saveAccountError("The account could not be connected. Try again.")
				return
			}
			saved := persistedAccountConnection{
				AccessToken: exchange.AccessToken, Email: exchange.Account.Email, Plan: exchange.Account.Plan,
				SubscriptionStatus: exchange.Account.SubscriptionStatus, GoogleLinked: exchange.Account.GoogleLinked,
			}
			if err := a.store.SetSetting(context.Background(), accountConnectionSetting, saved); err != nil {
				a.config.Logger.Error("persist account connection", "error", err)
				return
			}
			if err := a.syncAccount(context.Background()); err != nil {
				a.config.Logger.Warn("initial account sync failed", "error", err)
			}
			return
		}
	}
}

func (a *App) finishAccountPairing() {
	a.accountMu.Lock()
	a.accountPairing = false
	a.accountMu.Unlock()
}

func (a *App) saveAccountError(message string) {
	var saved persistedAccountConnection
	_, _ = a.store.GetSetting(context.Background(), accountConnectionSetting, &saved)
	saved.LastError = message
	_ = a.store.SetSetting(context.Background(), accountConnectionSetting, saved)
}

func (a *App) syncAccount(ctx context.Context) error {
	if a.accountClient == nil {
		return errors.New("account synchronization is unavailable")
	}
	a.accountMu.Lock()
	defer a.accountMu.Unlock()
	var saved persistedAccountConnection
	found, err := a.store.GetSetting(ctx, accountConnectionSetting, &saved)
	if err != nil {
		return err
	}
	if !found || saved.AccessToken == "" {
		return errors.New("connect an account before synchronizing")
	}
	account, err := a.accountClient.Session(ctx, saved.AccessToken)
	if err != nil {
		saved.LastError = "The account session needs to be connected again."
		_ = a.store.SetSetting(context.Background(), accountConnectionSetting, saved)
		return err
	}
	results, err := a.store.ListResults(ctx, 1000)
	if err != nil {
		return err
	}
	if err := a.accountClient.Upload(ctx, saved.AccessToken, results); err != nil {
		saved.LastError = "Measurements could not be synchronized. FiberPulse will retry automatically."
		_ = a.store.SetSetting(context.Background(), accountConnectionSetting, saved)
		return err
	}
	saved.Email = account.Email
	saved.Plan = account.Plan
	saved.SubscriptionStatus = account.SubscriptionStatus
	saved.GoogleLinked = account.GoogleLinked
	saved.LastSyncAt = time.Now().UTC()
	saved.LastError = ""
	return a.store.SetSetting(ctx, accountConnectionSetting, saved)
}

func (a *App) disconnectAccount(ctx context.Context) error {
	var saved persistedAccountConnection
	if found, err := a.store.GetSetting(ctx, accountConnectionSetting, &saved); err != nil {
		return err
	} else if found && saved.AccessToken != "" && a.accountClient != nil {
		if err := a.accountClient.Logout(ctx, saved.AccessToken); err != nil {
			a.config.Logger.Warn("revoke account device token", "error", err)
		}
	}
	return a.store.SetSetting(ctx, accountConnectionSetting, persistedAccountConnection{})
}

func (a *App) accountSyncLoop() {
	_ = a.syncAccount(a.ctx)
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			if err := a.syncAccount(a.ctx); err != nil {
				a.config.Logger.Debug("account synchronization skipped", "error", err)
			}
		}
	}
}
