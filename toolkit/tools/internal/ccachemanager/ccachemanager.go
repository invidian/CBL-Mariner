// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

// tools to to parse ccache configuration file

package ccachemanagerpkg

import (
	"context"
	"errors"
	"fmt"
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
	CommonGroupName = "common"
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

type CCacheArchive struct {
	LocalSourcePath  string
	RemoteSourcePath string
	LocalTargetPath  string
	RemoteTargetPath string
}

type CCacheManager struct {
	Configuration CCacheConfiguration
	RootCCacheDir string
	DownloadsDir  string
	UploadsDir    string
	// Package specific state
	PkgGroupName  string
	PkgGroupSize  int
	PkgArch       string
	PkgCCacheDir  string

	PkgTarFile    CCacheArchive
	PkgLabelFile  CCacheArchive
}

func loadConfiguration(configFileName string) (configuration CCacheConfiguration, err error) {

	logger.Log.Infof("  loading ccache configuration file: %s", configFileName)

	err = jsonutils.ReadJSONFile(configFileName, &configuration)
	if err != nil {
		logger.Log.Infof("Failed to load file. %v", err)
	} else {
		logger.Log.Infof("  Type           : %s", configuration.RemoteStoreConfig.Type)
		logger.Log.Infof("  TenantId       : %s", configuration.RemoteStoreConfig.TenantId)
		logger.Log.Infof("  UserName       : %s", configuration.RemoteStoreConfig.UserName)
		// logger.Log.Infof("  Password      : %s", configuration.RemoteStoreConfig.Password)
		logger.Log.Infof("  StorageAccount : %s", configuration.RemoteStoreConfig.StorageAccount)
		logger.Log.Infof("  ContainerName  : %s", configuration.RemoteStoreConfig.ContainerName)
		logger.Log.Infof("  Versionsfolder : %s", configuration.RemoteStoreConfig.VersionsFolder)
		logger.Log.Infof("  DownloadEnabled: %v", configuration.RemoteStoreConfig.DownloadEnabled)
		logger.Log.Infof("  DownloadFolder : %s", configuration.RemoteStoreConfig.DownloadFolder)
		logger.Log.Infof("  UploadEnabled  : %v", configuration.RemoteStoreConfig.UploadEnabled)
		logger.Log.Infof("  UploadFolder   : %s", configuration.RemoteStoreConfig.UploadFolder)
		logger.Log.Infof("  UpdateLatest   : %v", configuration.RemoteStoreConfig.UpdateLatest)
	}

	return configuration, err	
}

// Initialize() is called once per CCacheManager instance.
func (m *CCacheManager) Initialize(configFileName string, rootDir string) (err error) {

	logger.Log.Infof("Initialize(%s, %s)", configFileName, rootDir)

	logger.Log.Infof("  initializing ccache manager.")
	logger.Log.Infof("  ccache root folder         : (%s)", rootDir)
	logger.Log.Infof("  ccache remote configuration: (%s)", configFileName)

	m.Configuration, err = loadConfiguration(configFileName)
	if err != nil {
		logger.Log.Infof("Failed to load remote store configuration. %v", err)
		return err
	}

	if rootDir == "" {
		return errors.New("CCache root directory cannot be empty.")
	}

	m.RootCCacheDir = rootDir
	m.DownloadsDir = m.RootCCacheDir + "-downloads"
	m.UploadsDir = m.RootCCacheDir + "-uploads"

	logger.Log.Infof("  m.RootCCacheDir : (%s)", m.RootCCacheDir)
	return nil
}

func ensureDirExists(dirName string) (err error) {
	_, err = os.Stat(dirName)
	if err == nil {
		return nil
	}

	if os.IsNotExist(err) {
		err = os.Mkdir(dirName, 0755)
		if err != nil {
			logger.Log.Warnf("Unable to create folder (%s). Error: %v", dirName, err)
			return err
		}
	} else {
		logger.Log.Warnf("An error occured while checking if (%s) exists. Error: %v", dirName, err)
		return err
	}

	return nil
}

// SetPackage() is called once per package.
func (m *CCacheManager) SetPackage(basePackageName string, arch string) (err error) {
	groupName, groupSize := m.findGroup(basePackageName)

	return m.setPackageInternal(groupName, groupSize, arch)
}

// SetPackage() is called once per package.
func (m *CCacheManager) setPackageInternal(groupName string, groupSize int, arch string) (err error) {
	m.PkgGroupName = groupName
	m.PkgGroupSize = groupSize
	m.PkgArch = arch

	m.PkgCCacheDir, err = m.GetCCacheDir(m.PkgGroupName, m.PkgArch)
	if err != nil {
		return errors.New(fmt.Sprintf("Failed to construct the ccache directory name. Error (%v)", err))
	}
	logger.Log.Infof("  ccache working folder      : (%s)", m.PkgCCacheDir)

	CCacheTarSuffix := "-ccache.tar.gz"
	m.PkgTarFile.LocalSourcePath = m.DownloadsDir + "/" + m.PkgGroupName + CCacheTarSuffix
	m.PkgTarFile.RemoteSourcePath = m.PkgArch + "/" + m.Configuration.RemoteStoreConfig.DownloadFolder + "/" + m.PkgGroupName + CCacheTarSuffix
	m.PkgTarFile.LocalTargetPath = m.UploadsDir + "/" + m.PkgGroupName + CCacheTarSuffix
	m.PkgTarFile.RemoteTargetPath = m.PkgArch + "/" + m.Configuration.RemoteStoreConfig.UploadFolder + "/" + m.PkgGroupName + CCacheTarSuffix

	CCacheVersionSuffix := "-latest-build.txt"
	m.PkgLabelFile.LocalSourcePath = m.DownloadsDir + "/" + m.PkgGroupName + CCacheVersionSuffix
	m.PkgLabelFile.RemoteSourcePath = m.PkgArch + "/" + m.Configuration.RemoteStoreConfig.VersionsFolder + "/" + m.PkgGroupName + CCacheVersionSuffix
	m.PkgLabelFile.LocalTargetPath = m.UploadsDir + "/" + m.PkgGroupName + CCacheVersionSuffix
	m.PkgLabelFile.RemoteTargetPath = m.PkgArch + "/" + m.Configuration.RemoteStoreConfig.VersionsFolder + "/" + m.PkgGroupName + CCacheVersionSuffix

	return nil
}

// This function returns groupName="common" and groupSize=0 if any failure is
// encountered. This allows the ccachemanager to 'hide' the details of packages
// that are not part of any remote storage group.
func (m *CCacheManager) findGroup(basePackageName string) (groupName string, groupSize int) {

	groupName = ""
	groupSize = 0

	for _, group := range m.Configuration.Groups {
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

	if groupName == "" {
		logger.Log.Infof("  did not find ccache group for (%s) - assigning to group \"%s\".", basePackageName, CommonGroupName)
		groupName = CommonGroupName
		groupSize = 0
	}

	return groupName, groupSize
}

func (m *CCacheManager) findCCacheGroupSize(groupName string) (groupSize int) {

	groupSize = 0

	for _, group := range m.Configuration.Groups {
		if groupName == group.Name {
			groupSize = len(group.PackageNames)
		}
	}

	return groupSize
}

func (m *CCacheManager) GetCCacheDir(ccacheGroupName string, architecture string) (string, error) {
	if architecture == "" {
		return "", errors.New("CCache package architecture cannot be empty.")
	}
	if ccacheGroupName == "" {
		return "", errors.New("CCache package group name cannot be empty.")
	}
	return m.RootCCacheDir + "/" + architecture + "/" + ccacheGroupName, nil
}

func compressDir(sourceDir string, archiveName string) (err error) {

	// Ensure the output file does not exist...
	logger.Log.Infof("  removing older ccache tar output file (%s) if it exists...", archiveName)
	_, err = os.Stat(archiveName)
	if err == nil {
		logger.Log.Infof("  found ccache tar output file (%s). Removing...", archiveName)
		err = os.Remove(archiveName)
		if err != nil {
			logger.Log.Warnf("  unable to delete ccache out tar. Error: %v", err)
			return err
		}
	}

	// Create the archive...
	logger.Log.Infof("  compressing (%s) into (%s).", sourceDir, archiveName)
	compressStartTime := time.Now()
	tarArgs := []string{
		"cf",
		archiveName,
		"-C",
		sourceDir,
		"."}

	_, stderr, err := shell.Execute("tar", tarArgs...)
	if err != nil {
		logger.Log.Warnf("Unable compress ccache files itno archive. Error: %v", stderr)
		return err
	}
	compressEndTime := time.Now()
	logger.Log.Infof("  compress time: %s", compressEndTime.Sub(compressStartTime))	
	return nil
}

func uncompressFile(archiveName string, targetDir string) (err error) {
	logger.Log.Infof("  uncompressing (%s) into (%s).", archiveName, targetDir)
	uncompressStartTime := time.Now()
	tarArgs := []string{
		"xf",
		archiveName,
		"-C",
		targetDir,
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

func (m *CCacheManager) DownloadPkgGroupCCache() (err error) {

	logger.Log.Infof("  ccache is enabled --------------------")
	err = ensureDirExists(m.PkgCCacheDir)
	if err != nil {
		logger.Log.Warnf("Cannot create ccache download folder. Error: %v", err)
		return err
	}

	if m.PkgGroupName == CommonGroupName {
		logger.Log.Infof("  %s group - skipping download...", CommonGroupName)
		return nil
	}

	remoteStoreConfig := m.Configuration.RemoteStoreConfig
	if !remoteStoreConfig.DownloadEnabled {
		logger.Log.Infof("  downloading archived ccache artifacts is disabled. Skipping download...")
		return nil
	}

	logger.Log.Infof("  downloading and expanding...")
	err = ensureDirExists(m.DownloadsDir)
	if err != nil {
		logger.Log.Warnf("Cannot create ccache download folder. Error: %v", err)
		return err
	}

	logger.Log.Infof("  creating container client...")
	theClient, err := azureblobstorage.CreateClient(remoteStoreConfig.TenantId, remoteStoreConfig.UserName, remoteStoreConfig.Password, remoteStoreConfig.StorageAccount, azureblobstorage.AnonymousAccess)
	if err != nil {
		logger.Log.Warnf("Unable to init azure blob storage client. Error: %v", err)
		return err
	}

	if remoteStoreConfig.DownloadFolder == "latest" {

		logger.Log.Infof("  ccache is configured to use the latest...")

		// Download the versions file...
		logger.Log.Infof("  downloading (%s) to (%s)...", m.PkgLabelFile.RemoteSourcePath, m.PkgLabelFile.LocalSourcePath)
		err = azureblobstorage.Download(theClient, context.Background(), remoteStoreConfig.ContainerName, m.PkgLabelFile.RemoteSourcePath, m.PkgLabelFile.LocalSourcePath)
		if err != nil {
			logger.Log.Warnf("  unable to download ccache archive. Error: %v", err)
			return err
		}

		// Read the text contents...
		latestBuildLabel, err := ioutil.ReadFile(m.PkgLabelFile.LocalSourcePath)
		if err != nil {
			logger.Log.Warnf("Unable to read ccache version file contents. Error: %v", err)
			return err
		}

		// Adjust the download folder from 'latest' to the label loaded from the file...
		remoteStoreConfig.DownloadFolder = string(latestBuildLabel) 
		logger.Log.Infof("  ccache latest archive folder is (%s)...", remoteStoreConfig.DownloadFolder)
	}

	// Download the actual cache...
	logger.Log.Infof("  downloading (%s) to (%s)...", m.PkgTarFile.RemoteSourcePath, m.PkgTarFile.LocalSourcePath)
	err = azureblobstorage.Download(theClient, context.Background(), remoteStoreConfig.ContainerName, m.PkgTarFile.RemoteSourcePath, m.PkgTarFile.LocalSourcePath)
	if err != nil {
		logger.Log.Warnf("Unable to download ccache archive. Error: %v", err)
		return err
	}

	err = uncompressFile(m.PkgTarFile.LocalSourcePath, m.PkgCCacheDir)
	if err != nil {
		logger.Log.Warnf("Unable uncompress ccache files from archive. Error: %v", err)
		return err
	}

	return nil
}

func (m *CCacheManager) UploadPkgGroupCCache() (err error) {

	logger.Log.Infof("  ccache is enabled --------------------")
	if m.PkgGroupName == CommonGroupName {
		logger.Log.Infof("  %s group - skipping upload...", CommonGroupName)
		return nil
	}

    remoteStoreConfig := m.Configuration.RemoteStoreConfig
	if !remoteStoreConfig.UploadEnabled {
		logger.Log.Infof("  ccache update is disabled for this build.")
		return
	}

	logger.Log.Infof("  archiving and uploading...")
	err = ensureDirExists(m.UploadsDir)
	if err != nil {
		logger.Log.Warnf("Cannot create ccache download folder. Error: %v", err)
		return err
	}

	err = compressDir(m.PkgCCacheDir, m.PkgTarFile.LocalTargetPath)
	if err != nil {
		logger.Log.Warnf("Unable compress ccache files itno archive. Error: %v", err)
		return err
	}

	logger.Log.Infof("  connecting to azure storage blob...")
	theClient, err := azureblobstorage.CreateClient(remoteStoreConfig.TenantId, remoteStoreConfig.UserName, remoteStoreConfig.Password, remoteStoreConfig.StorageAccount, azureblobstorage.AuthenticatedAccess)
	if err != nil {
		logger.Log.Warnf("Unable create azure blob storage client. Error: %v", err)
		return err
	}

	// Upload the ccache archive
	logger.Log.Infof("  uploading ccache archive (%s) to (%s)...", m.PkgTarFile.LocalTargetPath, m.PkgTarFile.RemoteTargetPath)
	err = azureblobstorage.Upload(theClient, context.Background(), m.PkgTarFile.LocalTargetPath, remoteStoreConfig.ContainerName, m.PkgTarFile.RemoteTargetPath)
	if err != nil {
		logger.Log.Warnf("Unable to upload ccache archive. Error: %v", err)
		return err
	}

	// logger.Log.Infof("  removing ccache archive (%s)...", m.PkgTarFile.LocalTargetPath)
	// err := os.Remove(m.PkgTarFile.LocalTargetPath)
	// if err != nil {
	// 	logger.Log.Warnf("Unable to delete ccache archive (%s). Error: %v", m.PkgTarFile.LocalTargetPath, err)
	// }

	if remoteStoreConfig.UpdateLatest {
		// Create the latest label file...
		logger.Log.Infof("  creating a label file (%s) with content: (%s)...", m.PkgLabelFile.LocalTargetPath, remoteStoreConfig.UploadFolder)
		err = ioutil.WriteFile(m.PkgLabelFile.LocalTargetPath, []byte(remoteStoreConfig.UploadFolder), 0644)
		if err != nil {
			logger.Log.Warnf("Unable to write label information to temporary file. Error: %v", err)
			return err
		}

		// Upload the latest label file...
		logger.Log.Infof("  uploading label version (%s) to (%s)...", m.PkgLabelFile.LocalTargetPath, m.PkgLabelFile.RemoteTargetPath)
		err = azureblobstorage.Upload(theClient, context.Background(), m.PkgLabelFile.LocalTargetPath, remoteStoreConfig.ContainerName, m.PkgLabelFile.RemoteTargetPath)
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

// m.RootCCacheDir
//   <arch-1>
//     <groupName-1>
//     <groupName-2>
//   <arch-2>
//     <groupName-1>
//     <groupName-2>
//
func (m *CCacheManager) UploadAllPkgGroupCCaches() (err error) {

	architectures, err := getChildFolders(m.RootCCacheDir)
	errorsOccured := false
	if err != nil {
		return errors.New(fmt.Sprintf("failed to enumerate ccache child folders under (%s)...", m.RootCCacheDir))
	} 

	for _, architecture := range architectures {
		logger.Log.Infof("  found ccache architecture (%s)...", architecture)
		groupNames, err := getChildFolders(filepath.Join(m.RootCCacheDir, architecture))
		if err != nil {
			logger.Log.Warnf("failed to enumerate child folders under (%s)...", m.RootCCacheDir)
			errorsOccured = true
		} else {
			for _, groupName := range groupNames {
				logger.Log.Infof("  found group (%s)...", groupName)

				// Enable this continue only if we enable uploading as
				// soon as packages are done building.
				groupSize := m.findCCacheGroupSize(groupName)
				if groupSize < 2 {
					// This has either been processed earlier or there is
					// nothing to process.
					logger.Log.Infof("  group size is (%d). It has already been processed. Skipping...", groupSize)
					continue
				}

				groupCCacheDir, err := m.GetCCacheDir(groupName, architecture)
				if err != nil {
					logger.Log.Warnf("Failed to get ccache dir for architecture (%s) and group name (%s)...", architecture, groupName)
					errorsOccured = true
				}				
				logger.Log.Infof("  processing ccache folder (%s)...", groupCCacheDir)

				m.setPackageInternal(groupName, groupSize, architecture)

				err = m.UploadPkgGroupCCache()
				if err != nil {
					errorsOccured = true
					logger.Log.Warnf("CCache will not be archived for (%s) (%s)...", architecture, groupName)
				}
			}
		}
	}

	if errorsOccured {
		return errors.New("CCache archiving and upload failed. See above warning for more details.")
	}
	return nil
}