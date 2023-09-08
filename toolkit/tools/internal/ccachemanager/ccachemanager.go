// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// tools to to parse ccache configuration file

package ccachemanager

import (
	"context"
	"errors"
	"path/filepath"
	"io/ioutil"
	"os"
	"time"

	"github.com/microsoft/CBL-Mariner/toolkit/tools/internal/azureblobstorage"
	"github.com/microsoft/CBL-Mariner/toolkit/tools/internal/jsonutils"
	"github.com/microsoft/CBL-Mariner/toolkit/tools/internal/logger"
	"github.com/microsoft/CBL-Mariner/toolkit/tools/internal/shell"
)

const (
	CCacheVersionSuffix = "-latest-build.txt"
	CCacheTarSuffix = "-ccache.tar.gz"
)

type CCacheGroup struct {
	Name     string `json:"name"`
    PackageNames []string `json:"packageNames"`
}

type RemoteStoreConfig struct {
	Type            string `json:"type"`
	TenantId        string `json:"tenantId"`
	UserName        string `json:"userName"`
	Password        string `json:"password"`
	StorageAccount  string `json:"storageAccount"`
	ContainerName   string `json:"containerName"`
	VersionsFolder  string `json:"versionsFolder"`
	DownloadEnabled bool   `json:"downloadEnabled"`
	DownloadFolder  string `json:"downloadFolder"`
	UploadEnabled   bool   `json:"uploadEnabled"`
	UploadFolder    string `json:"uploadFolder"`
	UpdateLatest    bool   `json:"updateLatest"`
}

type CCacheConfiguration struct {
	RemoteStoreConfig RemoteStoreConfig `json:"remoteStore"`
	Groups            []CCacheGroup     `json:"groups"`
}

func GetCCacheRemoteStoreConfig(ccacheConfigFileName string) (remoteStoreConfig RemoteStoreConfig, err error) {

	// ccacheGroupsFile := "./resources/manifests/package/ccache_configuration.json"
	logger.Log.Infof("  loading ccache configuration file: %s", ccacheConfigFileName)
	var ccacheConfiguration CCacheConfiguration
	err = jsonutils.ReadJSONFile(ccacheConfigFileName, &ccacheConfiguration)
	if err != nil {
		logger.Log.Infof("Failed to load file. %v", err)
	} else {
		logger.Log.Infof("  Type           : %s", ccacheConfiguration.RemoteStoreConfig.Type)
		logger.Log.Infof("  TenantId       : %s", ccacheConfiguration.RemoteStoreConfig.TenantId)
		logger.Log.Infof("  UserName       : %s", ccacheConfiguration.RemoteStoreConfig.UserName)
		// logger.Log.Infof("  Password      : %s", ccacheConfiguration.RemoteStoreConfig.Password)
		logger.Log.Infof("  StorageAccount : %s", ccacheConfiguration.RemoteStoreConfig.StorageAccount)
		logger.Log.Infof("  ContainerName  : %s", ccacheConfiguration.RemoteStoreConfig.ContainerName)
		logger.Log.Infof("  Versionsfolder : %s", ccacheConfiguration.RemoteStoreConfig.VersionsFolder)
		logger.Log.Infof("  DownloadEnabled: %v", ccacheConfiguration.RemoteStoreConfig.DownloadEnabled)
		logger.Log.Infof("  DownloadFolder : %s", ccacheConfiguration.RemoteStoreConfig.DownloadFolder)
		logger.Log.Infof("  UploadEnabled  : %v", ccacheConfiguration.RemoteStoreConfig.UploadEnabled)
		logger.Log.Infof("  UploadFolder   : %s", ccacheConfiguration.RemoteStoreConfig.UploadFolder)
		logger.Log.Infof("  UpdateLatest   : %v", ccacheConfiguration.RemoteStoreConfig.UpdateLatest)
	}

	return ccacheConfiguration.RemoteStoreConfig, err	
}

func FindCCacheGroup(ccacheConfigFileName string, basePackageName string) (groupName string, groupSize int) {

	// ccacheGroupsFile := "resources/manifests/package/ccache_configuration.json"
	logger.Log.Infof("  loading ccache configuration file: %s", ccacheConfigFileName)
	var ccacheConfiguration CCacheConfiguration
	err := jsonutils.ReadJSONFile(ccacheConfigFileName, &ccacheConfiguration)
	if err != nil {
		logger.Log.Infof("Failed to load file. %v", err)
	} else {
		for _, group := range ccacheConfiguration.Groups {
			for _, packageName := range group.PackageNames {
				if packageName == basePackageName {
					logger.Log.Infof("  found group (%s) for base package (%s)...", group.Name, basePackageName)
					groupName = group.Name
					groupSize = len(group.PackageNames)
					break
				}
			}
			if groupName != "" {
				break
			}
		}
	}

	if groupName == "" {
		logger.Log.Infof("  did not find ccache group for (%s) - returning \"common\".", basePackageName)
		groupName = "common"
		groupSize = 0
	}

	return groupName, groupSize
}

func findCCacheGroupSize(ccacheConfigFileName string, groupName string) (groupSize int) {

	groupSize = 0

	// ccacheGroupsFile := "resources/manifests/package/ccache_configuration.json"
	logger.Log.Infof("  loading ccache configuration file: %s", ccacheConfigFileName)
	var ccacheConfiguration CCacheConfiguration
	err := jsonutils.ReadJSONFile(ccacheConfigFileName, &ccacheConfiguration)
	if err != nil {
		logger.Log.Infof("Failed to load file. %v", err)
	} else {
		for _, group := range ccacheConfiguration.Groups {
			if groupName == group.Name {
				groupSize = len(group.PackageNames)
			}
		}
	}

	return groupSize
}

func GetCCacheFolder(ccacheRootDir string, architecture string, ccacheGroupName string) (string) {
	return ccacheRootDir + "/" + architecture + "/" + ccacheGroupName
}

func InstallCCache(ccacheConfigFileName string, ccacheDir string, ccacheDirTarsIn string, ccacheGroupName string, architecture string) (err error) {

	logger.Log.Infof("  ccache is enabled --------------------")

	logger.Log.Infof("  checking if ccache working folder (%s) exists.", ccacheDir)
	_, err = os.Stat(ccacheDir)
	if err != nil {
		logger.Log.Infof("  creating ccache working folder...")
		err = os.MkdirAll(ccacheDir, 0755)
		if err != nil {
			logger.Log.Warnf("Unable to create ccache working folder. Error: %v", err)
			return err
		}
	} else {
		logger.Log.Infof("  ccache working folder (%s) does exist. Re-using...", ccacheDir)
		return nil
	}

	if ccacheGroupName == "common" {
		logger.Log.Infof("  common group - skipping download...")
		return nil
	}

	logger.Log.Infof("  downloading and expanding...")

	logger.Log.Infof("  retrieving remote store information...")
	remoteStoreConfig, err := GetCCacheRemoteStoreConfig(ccacheConfigFileName)
	if err != nil {
		logger.Log.Warnf("Unable to get ccache remote store configuration. Error: %v", err)
		return err
	}

	if !remoteStoreConfig.DownloadEnabled {
		logger.Log.Infof("  downloading archived ccache artifacts is disabled. Skipping download...")
		return nil
	}

	logger.Log.Infof("  checking if ccache archive download folder (%s) exists.", ccacheDirTarsIn)
	_, err = os.Stat(ccacheDirTarsIn)
	if err != nil {
		logger.Log.Infof("  creating ccache archive download folder...")
		err = os.Mkdir(ccacheDirTarsIn, 0755)
		if err != nil {
			logger.Log.Warnf("Unable to create ccache archive download folder. Error: %v", err)
			return err
		}
	}

	// Connect to blob storage...
	logger.Log.Infof("  creating container client...")
	theClient, err := azureblobstorage.CreateClient(remoteStoreConfig.TenantId, remoteStoreConfig.UserName, remoteStoreConfig.Password, remoteStoreConfig.StorageAccount, azureblobstorage.AnonymousAccess)
	if err != nil {
		logger.Log.Warnf("Unable to init azure blob storage client. Error: %v", err)
		return err
	}

	if remoteStoreConfig.DownloadFolder == "latest" {

		logger.Log.Infof("  ccache is configured to use the latest...")

		// Download the versions file...
		var ccacheVersionFullBlobName = architecture + "/" + remoteStoreConfig.VersionsFolder + "/" + ccacheGroupName + CCacheVersionSuffix
		var ccacheInputVersionFullPath = ccacheDirTarsIn + "/" + ccacheGroupName + CCacheVersionSuffix

		logger.Log.Infof("  downloading (%s) to (%s)...", ccacheVersionFullBlobName, ccacheInputVersionFullPath)
		err = azureblobstorage.Download(theClient, context.Background(), remoteStoreConfig.ContainerName, ccacheVersionFullBlobName, ccacheInputVersionFullPath)
		if err != nil {
			logger.Log.Warnf("  unable to download ccache archive. Error: %v", err)
			return err
		}

		// Read the text contents...
		ccacheInputVersionBuffer, err := ioutil.ReadFile(ccacheInputVersionFullPath)
		if err != nil {
			logger.Log.Warnf("Unable to read ccache version file contents. Error: %v", err)
			return err
		}

		// Adjust the download folder to the newly found value...
		remoteStoreConfig.DownloadFolder = string(ccacheInputVersionBuffer) 
		logger.Log.Infof("  ccache latest archive folder is (%s)...", remoteStoreConfig.DownloadFolder)
	}

	// Download the actual cache...
	var ccacheFullBlobName = architecture + "/" + remoteStoreConfig.DownloadFolder + "/" + ccacheGroupName + CCacheTarSuffix
	var ccacheInputTarFullPath = ccacheDirTarsIn + "/" + ccacheGroupName + CCacheTarSuffix

	logger.Log.Infof("  downloading (%s) to (%s)...", ccacheFullBlobName, ccacheInputTarFullPath)
	downloadStartTime := time.Now()
	err = azureblobstorage.Download(theClient, context.Background(), remoteStoreConfig.ContainerName, ccacheFullBlobName, ccacheInputTarFullPath)
	if err != nil {
		logger.Log.Warnf("Unable to download ccache archive. Error: %v", err)
		return err
	}
	downloadEndTime := time.Now()
	logger.Log.Infof("  download time: %v", downloadEndTime.Sub(downloadStartTime))

	logger.Log.Infof("  uncompressing (%s) into (%s).", ccacheInputTarFullPath, ccacheDir)
	uncompressStartTime := time.Now()
	tarArgs := []string{
		"xf",
		ccacheInputTarFullPath,
		"-C",
		ccacheDir,
		"."}

	_, stderr, err := shell.Execute("tar", tarArgs...)
	if err != nil {
		logger.Log.Warnf("Unable extract ccache files from archive. Error: %v", stderr)
		return err
	}
	uncompressEndTime := time.Now()
	logger.Log.Infof("  uncompress time: %v", uncompressEndTime.Sub(uncompressStartTime))

	return nil
}

func ArchiveCCache(ccacheConfigFileName string, ccacheDir string, ccacheDirTarsOut string, ccacheGroupName string, architecture string) (err error) {

	logger.Log.Infof("  ccache is enabled --------------------")

	if ccacheGroupName == "common" {
		logger.Log.Infof("  common group - skipping upload...")
		return nil
	}

	logger.Log.Infof("  archiving and uploading...")

    remoteStoreConfig, err := GetCCacheRemoteStoreConfig(ccacheConfigFileName)
	if err != nil {
		logger.Log.Warnf("Unable to get ccache remote store configuration. Error: %v", err)
		return err
	}

	if !remoteStoreConfig.UploadEnabled {
		logger.Log.Infof("  ccache update is disabled for this build.")
		return
	}

	// Ensure the output folder exists...
	logger.Log.Infof("  ensuring ccache tar output folder (%s) exists..", ccacheDirTarsOut)
	_, err = os.Stat(ccacheDirTarsOut)
	if err != nil {
		// TODO: george... use IsNotExist in other locations.
		if os.IsNotExist(err) {
			// If not, create it...
			err = os.Mkdir(ccacheDirTarsOut, 0755)
			if err != nil {
				logger.Log.Warnf("Unable create ccache out tar folder. Error: %v", err)
				return err
			}
		} else {
			logger.Log.Warnf("An error occured while check if ccache out tar folder exists. Error: %v", err)
			return err
		}
	}

	// Ensure the output file does not exist...
	ccacheOutputTarFullPath := ccacheDirTarsOut + "/" + ccacheGroupName + CCacheTarSuffix

	logger.Log.Infof("  removing older ccache tar output file (%s) if it exists...", ccacheOutputTarFullPath)
	_, err = os.Stat(ccacheOutputTarFullPath)
	if err == nil {
		logger.Log.Infof("  found ccache tar output file (%s). Removing...", ccacheOutputTarFullPath)
		err = os.Remove(ccacheOutputTarFullPath)
		if err != nil {
			logger.Log.Warnf("  unable to delete ccache out tar. Error: %v", err)
			return err
		}
	}

	// Create the archive...
	logger.Log.Infof("  compressing (%s) into (%s).", ccacheDir, ccacheOutputTarFullPath)
	compressStartTime := time.Now()
	tarArgs := []string{
		"cf",
		ccacheOutputTarFullPath,
		"-C",
		ccacheDir,
		"."}

	_, stderr, err := shell.Execute("tar", tarArgs...)
	if err != nil {
		logger.Log.Warnf("Unable compress ccache files itno archive. Error: %v", stderr)
		return err
	}
	compressEndTime := time.Now()
	logger.Log.Infof("  compress time: %s", compressEndTime.Sub(compressStartTime))

	// ** Temporary ** Uploading should take place at the end of the build
	// because other package family group members may update it.
	//

	logger.Log.Infof("  connecting to azure storage blob...")
	theClient, err := azureblobstorage.CreateClient(remoteStoreConfig.TenantId, remoteStoreConfig.UserName, remoteStoreConfig.Password, remoteStoreConfig.StorageAccount, azureblobstorage.AuthenticatedAccess)
	if err != nil {
		logger.Log.Warnf("Unable create azure blob storage client. Error: %v", stderr)
		return err
	}

	// Upload the ccache archive
	var ccacheFullBlobName = architecture + "/" + remoteStoreConfig.UploadFolder + "/" + ccacheGroupName + CCacheTarSuffix

	logger.Log.Infof("  uploading ccache archive (%s) to (%s)...", ccacheOutputTarFullPath, ccacheFullBlobName)

	err = azureblobstorage.Upload(theClient, context.Background(), ccacheOutputTarFullPath, remoteStoreConfig.ContainerName, ccacheFullBlobName)
	if err != nil {
		logger.Log.Warnf("Unable to upload ccache archive. Error: %v", err)
		return err
	}

	// logger.Log.Infof("  removing ccache archive (%s)...", ccacheOutputTarFullPath)
	// err := os.Remove(ccacheOutputTarFullPath)
	// if err != nil {
	// 	logger.Log.Warnf("Unable to delete ccache archive (%s). Error: %v", ccacheOutputTarFullPath, err)
	// }

	if remoteStoreConfig.UpdateLatest {
		// Create the latest version file...
		logger.Log.Infof("  creating a temporary version file with content: (%s)...", remoteStoreConfig.UploadFolder)

		tempFile, err := ioutil.TempFile("", ccacheGroupName + CCacheVersionSuffix + "-")
		if err != nil {
			logger.Log.Warnf("Unable to create temporary file to hold new version information. Error: %v", err)
			return err
		}
		defer tempFile.Close()

		_, err = tempFile.WriteString(remoteStoreConfig.UploadFolder)
		if err != nil {
			logger.Log.Warnf("Unable to write version information to temporary file. Error: %v", err)
			return err
		}

		// Upload the latest version file...
		var ccacheVersionFullBlobName = architecture + "/" + remoteStoreConfig.VersionsFolder + "/" + ccacheGroupName + CCacheVersionSuffix

		logger.Log.Infof("  uploading latest version (%s) to (%s)...", tempFile.Name(), ccacheVersionFullBlobName)
		err = azureblobstorage.Upload(theClient, context.Background(), tempFile.Name(), remoteStoreConfig.ContainerName, ccacheVersionFullBlobName)
		if err != nil {
			logger.Log.Warnf("Unable to upload ccache archive. Error: %v", err)
			return err
		}
	}

	return nil
}

func getChildFolders(parentFolder string) ([]string, error) {
	childFolders := []string{}

	dir, err := os.Open(parentFolder)
	if err != nil {
		logger.Log.Infof("  error opening parent folder. Error: (%v)", err)
		return nil, err
	}
	defer dir.Close()

	children, err := dir.Readdirnames(-1)
	if err != nil {
		logger.Log.Infof("  error enumerating children. Error: (%v)", err)
		return nil, err
	}

	for _, child := range children {
		childPath := filepath.Join(parentFolder, child)

		info, err := os.Stat(childPath)
		if err != nil {
			logger.Log.Infof("  error retrieving child attributes. Error: (%v)", err)
			continue
		}

		if info.IsDir() {
			childFolders = append(childFolders, child)
		}
	}

	return childFolders, nil
}

// ccacheRootDir
//   <arch-1>
//     <groupName-1>
//     <groupName-2>
//   <arch-2>
//     <groupName-1>
//     <groupName-2>
//
func ArchiveCCacheAll(ccacheConfigFileName string, ccacheRootDir string) (err error) {

	ccacheDirTarsOut := ccacheRootDir + "-tars-out"
	architectures, err := getChildFolders(ccacheRootDir)
	errorsOccured := false
	if err != nil {
		logger.Log.Warnf("failed to enumerate child folders under (%s)...", ccacheRootDir)
		errorsOccured = true
	} else {
		for _, architecture := range architectures {
			logger.Log.Infof("  found ccache architecture (%s)...", architecture)
			groupNames, err := getChildFolders(filepath.Join(ccacheRootDir, architecture))
			if err != nil {
				logger.Log.Warnf("failed to enumerate child folders under (%s)...", ccacheRootDir)
				errorsOccured = true
			} else {
				for _, groupName := range groupNames {
					logger.Log.Infof("  found group (%s)...", groupName)

					// Enable this continue only if we enable uploading as
					// soon as packages are done building.
					groupSize := findCCacheGroupSize(ccacheConfigFileName, groupName)
					if groupSize < 2 {
						// This has either been processed earlier or there is
						// nothing to process.
						logger.Log.Infof("  group size is (%d) - skipping...", groupSize)
						continue
					}

					groupCCacheDir := GetCCacheFolder(ccacheRootDir, architecture, groupName)
					logger.Log.Infof("  processing ccache folder (%s)...", groupCCacheDir)

					err = ArchiveCCache(ccacheConfigFileName, groupCCacheDir, ccacheDirTarsOut, groupName, architecture)
					if err != nil {
						errorsOccured = true
						logger.Log.Warnf("CCache will not be archived for (%s) (%s)...", architecture, groupName)
					}
				}
			}
		}
	}

	if errorsOccured {
		return errors.New("CCache archiving and upload failed. See above warning for more details.")
	}
	return nil
}