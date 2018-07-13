// Copyright (c) 2017 Zededa, Inc.
// All rights reserved.

// base os event handlers

package zedagent

import (
	"errors"
	"fmt"
	"github.com/zededa/go-provision/cast"
	"github.com/zededa/go-provision/types"
	"github.com/zededa/go-provision/zboot"
	"log"
	"os"
	"reflect"
	"time"
)

// the storage download/verification event handler
// through base of storage list
func baseOsHandleStatusUpdateSafename(ctx *zedagentContext, safename string) {

	log.Printf("baseOsStatusUpdateSafename for %s\n", safename)

	items := ctx.subBaseOsConfig.GetAll()
	for _, c := range items {
		baseOsConfig := cast.CastBaseOsConfig(c)
		for _, sc := range baseOsConfig.StorageConfigList {

			safename1 := types.UrlToSafename(sc.DownloadURL, sc.ImageSha256)

			// base os config contains the current image
			if safename == safename1 {

				uuidStr := baseOsConfig.Key()
				log.Printf("%s, found baseOs %s\n", safename, uuidStr)

				// handle the change event for this base os config
				baseOsHandleStatusUpdate(ctx, uuidStr)
			}
		}
	}
}

func addOrUpdateBaseOsConfig(ctx *zedagentContext, uuidStr string,
	config types.BaseOsConfig) {

	changed := false
	added := false

	if m := lookupBaseOsConfig(ctx, uuidStr); m != nil {
		// XXX or just compare version like elsewhere?
		// XXX switch to Equal?
		if !reflect.DeepEqual(m, config) {
			log.Printf("addOrUpdateBaseOsConfig(%v) for %s, Config change\n",
				config.BaseOsVersion, uuidStr)
			changed = true
		} else {
			log.Printf("addOrUpdateBaseOsConfig(%v) for %s, No change\n",
				config.BaseOsVersion, uuidStr)
		}
	} else {
		log.Printf("addOrUpdateBaseOsConfig(%v) for %s, Config add\n",
			config.BaseOsVersion, uuidStr)
		added = true
		changed = true
	}

	if added {

		status := types.BaseOsStatus{
			UUIDandVersion: config.UUIDandVersion,
			BaseOsVersion:  config.BaseOsVersion,
			ConfigSha256:   config.ConfigSha256,
			PartitionLabel: config.PartitionLabel,
		}

		status.StorageStatusList = make([]types.StorageStatus,
			len(config.StorageConfigList))

		for i, sc := range config.StorageConfigList {
			ss := &status.StorageStatusList[i]
			ss.DownloadURL = sc.DownloadURL
			ss.ImageSha256 = sc.ImageSha256
			ss.Target = sc.Target
		}

		baseOsGetActivationStatus(&status)
		publishBaseOsStatus(ctx, &status)
	}

	if changed {
		baseOsHandleStatusUpdate(ctx, uuidStr)
	}
}

func baseOsGetActivationStatus(status *types.BaseOsStatus) {
	log.Printf("baseOsGetActivationStatus(%s): partitionLabel %s\n",
		status.BaseOsVersion, status.PartitionLabel)

	// PartitionLabel can be empty here!
	if status.PartitionLabel == "" {
		status.Activated = false
		return
	}

	partName := status.PartitionLabel

	// some partition specific attributes
	status.PartitionState = zboot.GetPartitionState(partName)
	status.PartitionDevice = zboot.GetPartitionDevname(partName)

	// for otherPartition, its always false
	if !zboot.IsCurrentPartition(partName) {
		status.Activated = false
		return
	}
	// if current Partition, get the status from zboot
	status.Activated = zboot.IsCurrentPartitionStateActive()
}

func baseOsHandleStatusUpdate(ctx *zedagentContext, uuidStr string) {

	config := lookupBaseOsConfig(ctx, uuidStr)
	if config == nil {
		return
	}

	status := lookupBaseOsStatus(ctx, uuidStr)
	if status == nil {
		return
	}

	baseOsGetActivationStatus(status)

	changed := doBaseOsStatusUpdate(ctx, uuidStr, *config, status)

	if changed {
		log.Printf("baseOsHandleStatusUpdate(%s) for %s, Status changed\n",
			config.BaseOsVersion, uuidStr)
		publishBaseOsStatus(ctx, status)
	}
}

func doBaseOsStatusUpdate(ctx *zedagentContext, uuidStr string,
	config types.BaseOsConfig, status *types.BaseOsStatus) bool {

	log.Printf("doBaseOsStatusUpdate(%s) for %s\n",
		config.BaseOsVersion, uuidStr)

	changed, proceed := doBaseOsInstall(ctx, uuidStr, config, status)
	if !proceed {
		return changed
	}

	if config.Activate == false {
		log.Printf("doBaseOsStatusUpdate(%s) for %s, Activate is not set\n",
			config.BaseOsVersion, uuidStr)
		changed = doBaseOsInactivate(uuidStr, status)
		return changed
	}

	if status.Activated == true {
		log.Printf("doBaseOsStatusUpdate(%s) for %s, is already activated\n",
			config.BaseOsVersion, uuidStr)
		return false
	}

	changed = doBaseOsActivate(uuidStr, config, status)
	log.Printf("doBaseOsStatusUpdate(%s) done for %s\n",
		config.BaseOsVersion, uuidStr)
	return changed
}

// Returns changed boolean when the status was changed
func doBaseOsActivate(uuidStr string, config types.BaseOsConfig,
	status *types.BaseOsStatus) bool {
	log.Printf("doBaseOsActivate(%s) uuid %s\n",
		config.BaseOsVersion, uuidStr)

	changed := false
	log.Printf("doBaseOsActivate(%s) for %s, partition %s\n",
		config.BaseOsVersion, uuidStr, config.PartitionLabel)

	if config.PartitionLabel == "" {
		log.Printf("doBaseOsActivate(%s) for %s, unassigned partition\n",
			config.BaseOsVersion, uuidStr)
		return changed
	}

	// Sanity check the partition label of the current root and
	// the partition state
	// We've already dd'ed the new image into the partition
	// hence can't compare versions here. Version check was done when
	// processing the baseOsConfig.

	if !zboot.IsOtherPartition(config.PartitionLabel) {
		return changed
	}
	partState := zboot.GetPartitionState(config.PartitionLabel)
	switch partState {
	case "unused":
		log.Printf("Installing %s over unused\n",
			config.BaseOsVersion)
	case "inprogress":
		log.Printf("Installing %s over inprogress\n",
			config.BaseOsVersion)
	case "updating":
		log.Printf("Installing %s over updating\n",
			config.BaseOsVersion)
	default:
		// XXX we seem to hit this in some cases
		// Happens when a new baseOs config appears while
		// we are still testing the previous update in
		// which case the current is inprogress and the other/fallback
		// partition is active.
		errString := fmt.Sprintf("Wrong partition state %s for %s",
			partState, config.PartitionLabel)
		log.Println(errString)
		status.Error = errString
		status.ErrorTime = time.Now()
		changed = true
		return changed
	}

	log.Printf("doBaseOsActivate: %s activating\n", uuidStr)
	zboot.SetOtherPartitionStateUpdating()
	publishDeviceInfo = true

	// Remove any old log files for a previous instance
	logdir := fmt.Sprintf("/persist/%s/log", config.PartitionLabel)
	log.Printf("Clearing old logs in %s\n", logdir)
	if err := os.RemoveAll(logdir); err != nil {
		log.Println(err)
	}

	// if it is installed, flip the activated status
	if status.State == types.INSTALLED ||
		status.Activated == false {
		status.Activated = true
		changed = true
		// Make sure we tell apps to shut down
		shutdownAppsGlobal()
		startExecReboot()
	}

	return changed
}

func doBaseOsInstall(ctx *zedagentContext, uuidStr string,
	config types.BaseOsConfig,
	status *types.BaseOsStatus) (bool, bool) {

	log.Printf("%s, doBaseOsInstall(%s) \n", uuidStr, config.BaseOsVersion)
	changed := false
	proceed := false

	for i, sc := range config.StorageConfigList {
		ss := &status.StorageStatusList[i]
		if ss.DownloadURL != sc.DownloadURL ||
			ss.ImageSha256 != sc.ImageSha256 {
			// Report to zedcloud
			errString := fmt.Sprintf("%s, for %s, Storage config mismatch:\n\t%s\n\t%s\n\t%s\n\t%s\n\n", uuidStr,
				config.BaseOsVersion,
				sc.DownloadURL, ss.DownloadURL,
				sc.ImageSha256, ss.ImageSha256)
			log.Println(errString)
			status.Error = errString
			status.ErrorTime = time.Now()
			changed = true
			return changed, proceed
		}
	}

	// check for the download status change
	downloadchange, downloaded :=
		checkBaseOsStorageDownloadStatus(uuidStr, config, status)

	if downloaded == false {
		log.Printf(" %s, Still not downloaded\n", config.BaseOsVersion)
		return changed || downloadchange, proceed
	}

	// check for the verification status change
	verifychange, verified :=
		checkBaseOsVerificationStatus(uuidStr, config, status)

	if verified == false {
		log.Printf("%s, Still not verified\n", config.BaseOsVersion)
		return changed || verifychange, proceed
	}

	// install the image at proper partition
	if ret := installDownloadedObjects(baseOsObj, uuidStr,
		config.StorageConfigList, status.StorageStatusList); ret == true {

		changed = true
		//match the version string
		if errString := checkInstalledVersion(config); errString != "" {
			status.State = types.INITIAL
			status.Error = errString
			status.ErrorTime = time.Now()
		} else {
			// move the state from DELIVERED to INSTALLED
			status.State = types.INSTALLED
			proceed = true
		}
	}

	publishBaseOsStatus(ctx, status)
	log.Printf("doBaseOsInstall(%s), Done %v\n",
		config.BaseOsVersion, proceed)
	return changed, proceed
}

func checkBaseOsStorageDownloadStatus(uuidStr string,
	config types.BaseOsConfig,
	status *types.BaseOsStatus) (bool, bool) {

	log.Printf("checkBaseOsStorageDownloadStatus(%s) for %s\n",
		config.BaseOsVersion, uuidStr)
	ret := checkStorageDownloadStatus(baseOsObj, uuidStr,
		config.StorageConfigList, status.StorageStatusList)

	status.State = ret.MinState

	if ret.MinState == types.INITIAL {
		status.Error = ret.AllErrors
		status.ErrorTime = ret.ErrorTime
		log.Printf("checkBaseOsStorageDownloadStatus(%s) for %s, Download error at %v: %v\n",
			config.BaseOsVersion, uuidStr, status.ErrorTime, status.Error)
		return ret.Changed, false
	}

	if ret.MinState < types.DOWNLOADED {
		log.Printf("checkBaseOsStorageDownloadStatus(%s) for %s, Waiting for all downloads\n",
			config.BaseOsVersion, uuidStr)
		return ret.Changed, false
	}

	if ret.WaitingForCerts {
		log.Printf("checkBaseOsStorageDownloadStatus(%s) for %s, Waiting for certs\n",
			config.BaseOsVersion, uuidStr)
		return ret.Changed, false
	}

	log.Printf("checkBaseOsStorageDownloadStatus(%s) for %s, Downloads done\n",
		config.BaseOsVersion, uuidStr)
	return ret.Changed, true
}

func checkBaseOsVerificationStatus(uuidStr string,
	config types.BaseOsConfig, status *types.BaseOsStatus) (bool, bool) {

	ret := checkStorageVerifierStatus(baseOsObj,
		uuidStr, config.StorageConfigList, status.StorageStatusList)

	status.State = ret.MinState

	if ret.MinState == types.INITIAL {
		status.Error = ret.AllErrors
		status.ErrorTime = ret.ErrorTime
		log.Printf("checkBaseOsVerificationStatus(%s) for %s, Verification error at %v: %v\n",
			config.BaseOsVersion, uuidStr, status.ErrorTime, status.Error)
		return ret.Changed, false
	}

	if ret.MinState < types.DELIVERED {
		log.Printf("checkBaseOsVerificationStatus(%s) for %s, Waiting for all verifications\n",
			config.BaseOsVersion, uuidStr)
		return ret.Changed, false
	}
	log.Printf("checkBaseOsVerificationStatus(%s) for %s, Verifications done\n",
		config.BaseOsVersion, uuidStr)
	return ret.Changed, true
}

func removeBaseOsConfig(ctx *zedagentContext, uuidStr string) {

	log.Printf("removeBaseOsConfig for %s\n", uuidStr)
	removeBaseOsStatus(ctx, uuidStr)
	log.Printf("removeBaseOSConfig for %s, done\n", uuidStr)
}

func removeBaseOsStatus(ctx *zedagentContext, uuidStr string) {

	log.Printf("removeBaseOsStatus for %s\n", uuidStr)
	status := lookupBaseOsStatus(ctx, uuidStr)
	if status == nil {
		log.Printf("removeBaseOsStatus: no status\n")
		return
	}

	changed, del := doBaseOsRemove(uuidStr, status)
	if changed {
		log.Printf("removeBaseOsStatus for %s, Status change\n", uuidStr)
		publishBaseOsStatus(ctx, status)
	}

	if del {
		log.Printf("removeBaseOsStatus %s, Deleting\n", uuidStr)

		// Write out what we modified to BaseOsStatus aka delete
		unpublishBaseOsStatus(ctx, status.Key())
	}
	log.Printf("removeBaseOsStatus %s, Done\n", uuidStr)
}

func doBaseOsRemove(uuidStr string, status *types.BaseOsStatus) (bool, bool) {

	log.Printf("doBaseOsRemove(%s) for %s\n", status.BaseOsVersion, uuidStr)

	changed := false
	del := false

	changed = doBaseOsInactivate(uuidStr, status)

	changed, del = doBaseOsUninstall(uuidStr, status)

	log.Printf("doBaseOsRemove(%s) for %s, Done\n",
		status.BaseOsVersion, uuidStr)
	return changed, del
}

func doBaseOsInactivate(uuidStr string, status *types.BaseOsStatus) bool {
	log.Printf("doBaseOsInactivate(%s) %v\n",
		status.BaseOsVersion, status.Activated)

	// nothing to be done, flip will happen on reboot
	return true
}

func doBaseOsUninstall(uuidStr string, status *types.BaseOsStatus) (bool, bool) {
	log.Printf("doBaseOsUninstall(%s) for %s\n",
		status.BaseOsVersion, uuidStr)

	del := false
	changed := false
	removedAll := true

	// If this image is on the !active partition we mark that
	// as unused.
	if status.PartitionLabel != "" {
		partName := status.PartitionLabel
		if status.BaseOsVersion == zboot.GetShortVersion(partName) &&
			zboot.IsOtherPartition(partName) {
			log.Printf("doBaseOsUninstall(%s) for %s, currently on other %s\n",
				status.BaseOsVersion, uuidStr, partName)
			log.Printf("Mark other partition %s, unused\n", partName)
			zboot.SetOtherPartitionStateUnused()
		}
	}
	for i, _ := range status.StorageStatusList {

		ss := &status.StorageStatusList[i]

		// Decrease refcount if we had increased it
		if ss.HasVerifierRef {
			log.Printf("doBaseOsUninstall(%s) for %s, HasVerifierRef %s\n",
				status.BaseOsVersion, uuidStr, ss.ImageSha256)
			MaybeRemoveVerifierConfigSha256(baseOsObj,
				ss.ImageSha256)
			ss.HasVerifierRef = false
			changed = true
		} else {
			log.Printf("doBaseOsUninstall(%s) for %s, NO HasVerifier\n",
				status.BaseOsVersion, uuidStr)
		}

		vs, err := lookupVerificationStatusSha256(baseOsObj,
			ss.ImageSha256)

		if err == nil {
			log.Printf("doBaseOsUninstall(%s) for %s, Verifier %s not yet gone; RefCount %d\n",
				status.BaseOsVersion, uuidStr, ss.ImageSha256,
				vs.RefCount)
			removedAll = false
			continue
		}
	}

	if !removedAll {
		// XXX not that we hit this all the time, and
		// we proceed to not look at the downloads and proceed
		// to delete all the config and status for this baseos, which
		// is odd.
		// Changed to proceed in any case
		log.Printf("doBaseOsUninstall(%s) for %s, NOT Waiting for verifier purge\n",
			status.BaseOsVersion, uuidStr)
		// XXX return changed, del
	}

	removedAll = true

	for i, _ := range status.StorageStatusList {

		ss := &status.StorageStatusList[i]
		safename := types.UrlToSafename(ss.DownloadURL, ss.ImageSha256)
		// Decrease refcount if we had increased it
		if ss.HasDownloaderRef {
			log.Printf("doBaseOsUninstall(%s) for %s, HasDownloaderRef %s\n",
				status.BaseOsVersion, uuidStr, safename)

			removeDownloaderConfig(baseOsObj, safename)
			ss.HasDownloaderRef = false
			changed = true
		} else {
			log.Printf("doBaseOsUninstall(%s) for %s, NO HasDownloaderRef\n",
				status.BaseOsVersion, uuidStr)
		}

		ds, err := lookupDownloaderStatus(baseOsObj, ss.ImageSha256)
		if err == nil {
			log.Printf("doBaseOsUninstall(%s) for %s, Download %s not yet gone; RefCount %d\n",
				status.BaseOsVersion, uuidStr, safename,
				ds.RefCount)
			removedAll = false
			continue
		}
	}

	if !removedAll {
		log.Printf("doBaseOsUninstall(%s) for %s, Waiting for downloader purge\n",
			status.BaseOsVersion, uuidStr)
		return changed, del
	}

	del = true
	log.Printf("doBaseOsUninstall(%s), Done\n", status.BaseOsVersion)
	return changed, del
}

func installBaseOsObject(srcFilename string, dstFilename string) error {

	log.Printf("installBaseOsObject: %s to %s\n", srcFilename, dstFilename)

	if dstFilename == "" {
		log.Printf("installBaseOsObject: unssigned destination partition\n")
		err := errors.New("no destination partition")
		return err
	}

	err := zboot.WriteToPartition(srcFilename, dstFilename)
	if err != nil {
		log.Printf("installBaseOsObject: write failed %s\n", err)
		return err
	}
	return nil
}

// validate whether the image version matches with
// config version string
func checkInstalledVersion(config types.BaseOsConfig) string {

	log.Printf("%s, check baseOs installation %s\n",
		config.PartitionLabel, config.BaseOsVersion)

	if config.PartitionLabel == "" {
		errStr := fmt.Sprintf("%s, invalid partition", config.BaseOsVersion)
		log.Println(errStr)
		return errStr
	}

	partVersion := zboot.GetShortVersion(config.PartitionLabel)
	// XXX this check can result in failures when multiple updates in progress in zedcloud!
	// XXX remove?
	if config.BaseOsVersion != partVersion {
		errStr := fmt.Sprintf("baseOs %s, %s, does not match installed %s",
			config.PartitionLabel, config.BaseOsVersion, partVersion)

		log.Println(errStr)
		// XXX return errStr
	}
	return ""
}

func lookupBaseOsConfig(ctx *zedagentContext, key string) *types.BaseOsConfig {

	sub := ctx.subBaseOsConfig
	c, _ := sub.Get(key)
	if c == nil {
		log.Printf("lookupBaseOsConfig(%s) not found\n", key)
		return nil
	}
	config := cast.CastBaseOsConfig(c)
	if config.Key() != key {
		log.Printf("lookupBaseOsConfig(%s) got %s; ignored %+v\n",
			key, config.Key(), config)
		return nil
	}
	return &config
}

func lookupBaseOsStatus(ctx *zedagentContext, key string) *types.BaseOsStatus {
	pub := ctx.pubBaseOsStatus
	st, _ := pub.Get(key)
	if st == nil {
		log.Printf("lookupBaseOsStatus(%s) not found\n", key)
		return nil
	}
	status := cast.CastBaseOsStatus(st)
	if status.Key() != key {
		log.Printf("lookupBaseOsStatus(%s) got %s; ignored %+v\n",
			key, status.Key(), status)
		return nil
	}
	return &status
}

func publishBaseOsStatus(ctx *zedagentContext, status *types.BaseOsStatus) {

	key := status.Key()
	log.Printf("Publishing BaseOsStatus %s\n", key)
	pub := ctx.pubBaseOsStatus
	pub.Publish(key, status)
}

func unpublishBaseOsStatus(ctx *zedagentContext, key string) {

	log.Printf("Unpublishing BaseOsStatus %s\n", key)
	pub := ctx.pubBaseOsStatus
	st, _ := pub.Get(key)
	if st == nil {
		log.Printf("unpublishBaseOsStatus(%s) not found\n", key)
		return
	}
	pub.Unpublish(key)
}
