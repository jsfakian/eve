# Application Snapshot, Implementation Details

## Overview

This document is intended for developers and contributors to the EVE project who
are interested in the technical aspects of the snapshotting feature. It delves
into the nitty-gritty details of how the feature is implemented in the EVE
codebase. Topics covered include:

* **Data Structures**: Definitions and explanations of the key data structures
  used.
* **Flow of Operations**: Step-by-step walkthrough of the snapshot creation,
  rollback, and deletion processes.
* **Serialization and Deserialization**: How snapshot data is stored and
  retrieved, including the use of mandatory fields for version compatibility.
* **Low-Level Filesystem Operations**: How the snapshotting feature interacts
  with the filesystem to perform snapshot operations.
* **Error Handling**: How errors are handled and reported during snapshot
  operations.
* **Testing**: How the snapshotting feature is tested, including unit tests and
  integration tests.

This document assumes that the reader is familiar with the general functionality
and API of the snapshotting feature, as described in the Feature API Document in
[docs/APP-SNAPSHOT.md](../../../docs/APP-SNAPSHOT.md).

## EVE Agents

The snapshotting feature is implemented in several agents, including:

* **[zedagent](../cmd/zedagent)**: Parses and reports
  snapshot-related information from/to the controller.
* **[zedmanager](../cmd/zedmanager)**: Manages the application
  lifecycle, including snapshot-related operations during application
  installation, modification, and deletion.
* **[volumemgr](../cmd/volumemgr)**: Handles the creation,
  modification, and deletion of volume snapshots.
* **[utils](../utils)** Package: Contains utility functions for
  serialization and deserialization of snapshot data structures.

## Snapshot-Related Data Flow

The data flow in the system involves interactions between the controller,
zedagent, zedmanager, and volumemgr. The controller is responsible for sending
configuration updates and commands to the device. The zedagent acts as an
intermediary between the controller and the other agents on the device. It
parses the configuration updates and commands received from the controller and
forwards them to the appropriate agents. The zedmanager is responsible for
managing app instances and handling any associated requests. It interacts with
the volumemgr, which manages volumes and handles volume-related requests. The
agents process the requests and send status updates back to the zedagent, which
reports the updated status back to the controller.

1. **Controller to Zedagent**: The controller sends requests to the zedagent,
   which can include requests for snapshot creation, rollback, or deletion, as
   well as other app instance-related requests. The zedagent is responsible for
   parsing these requests and passing the relevant information to the
   zedmanager.
    * [cmd/zedagent/parseconfig.go](../cmd/zedagent/parseconfig.go)
2. **Zedagent to Zedmanager**: The zedagent passes the parsed app instance
   config and any associated requests to the zedmanager. The zedmanager is
   responsible for managing app instances and handling any associated requests,
   including those related to snapshots.
    * [cmd/zedagent/parseconfig.go](../cmd/zedagent/parseconfig.go)
3. **Zedmanager: App Lifecycle And Actions Triggering** \
   The zedmanager processes the requests and updates the `AppInstanceStatus`,
   which includes the `SnapshottingStatus`. The `SnapshottingStatus` contains
   information about requested snapshots, available snapshots, snapshots to be
   deleted, and other snapshot-related information. The zedmanager handles the
   requests by initiating the appropriate processes, such as the purge process
   for snapshot creation or the rollback process for snapshot rollback.
    * [cmd/zedmanager/zedmanager.go](../cmd/zedmanager/zedmanager.go)
    * [cmd/zedmanager/updatestatus.go](../cmd/zedmanager/updatestatus.go)
4. **Zedmanager to Volumemgr**: For snapshot-related requests, the zedmanager
   sends a VolumesSnapshotConfig to the volumemgr. This config includes
   information about the snapshot request, such as the snapshot ID, action,
   volume IDs, and app UUID. The volumemgr is responsible for managing volumes
   and handling volume-related requests, including those related to snapshots.
    * [cmd/zedmanager/updatestatus.go](../cmd/zedmanager/updatestatus.go)
    * [cmd/zedmanager/handlevolumemgr.go](../cmd/zedmanager/handlevolumemgr.go)
    * [cmd/volumemgr/volumemgr.go](../cmd/volumemgr/volumemgr.go)
5. **Volumemgr**: The volumemgr processes the `VolumesSnapshotConfig` and
   handles the creation, rollback, or deletion of the snapshot based on the
   action in the config. It interacts with the file system (FS)-specific
   handlers to perform the requested action and stores the relevant information
   in the VolumesSnapshotStatus.
    * [cmd/volumemgr/handlesnapshot.go](../cmd/volumemgr/handlesnapshot.go)
    * [volumehandlers](../volumehandlers)
    * [diskmetrics/diskmetrics.go](../diskmetrics/diskmetrics.go)
6. **Volumemgr to Zedmanager**: Once the snapshot action is performed, the
   volumemgr sends a VolumesSnapshotStatus back to the zedmanager. This status
   includes information about the snapshot, such as the snapshot ID, metadata,
   time created, app UUID, reference count, the result of the action, and error
   information. Sometimes to report an error on this step, a dedicated status
   can be created. It’s done when there is no existing one.
    * [cmd/volumemgr/handlesnapshot.go](../cmd/volumemgr/handlesnapshot.go)
7. **Zedmanager: Reaction** \
   The zedmanager receives the VolumesSnapshotStatus from the volumemgr and
   updates the SnapshottingStatus in the AppInstanceStatus. It handles any
   further actions based on the updated status and reports the updated status
   back to the zedagent.
    * [cmd/zedmanager/handlevolumemgr.go](../cmd/zedmanager/handlevolumemgr.go)
    * [cmd/zedmanager/updatestatus.go](../cmd/zedmanager/updatestatus.go)
8. **Zedagent to Controller**: The zedagent receives the updated
   AppInstanceStatus from the zedmanager and reports the updated status,
   including the SnapshottingStatus, back to the controller.
    * [cmd/zedagent/handlemetrics.go](../cmd/zedagent/handlemetrics.go)

This data flow ensures that the requests from the controller are processed and
handled by the appropriate agents and that the controller is informed of the
updated status of the app instance and any associated snapshots. The stored
information in the VolumesSnapshotStatus and SnapshottingStatus can be used for
future operations related to app instances and snapshots.

## Snapshot Data Structures

The snapshot creation process relies on several data structures to manage and
track the status of snapshots for each application instance. These data
structures are part of the `AppInstanceStatus` and are used
to handle snapshot requests, creation, deletion, and rollback.

### SnapshottingStatus

This is the main data structure that manages and tracks all snapshot-related
activities for an app instance. It is included in the AppInstanceStatus
structure. It contains fields that track the state of snapshots for the app
instance, such as RequestedSnapshots, AvailableSnapshots, and
SnapshotsToBeDeleted. It also includes fields that track the state of
snapshot-related actions, such as SnapshotOnUpgrade, HasRollbackRequest,
ActiveSnapshot, and RollbackInProgress.

It is updated by the zedmanager based on snapshot requests from the controller,
snapshot status from the volumemgr, and snapshot-related actions performed by
the zedmanager.

Defined
in [types/zedmanagertypes.go](../types/zedmanagertypes.go).

### SnapshotInstanceStatus

This structure represents the status of a snapshot instance at the zedmanager
level. It contains information about the snapshot, such as its ID, type, and
creation time. It also includes fields for tracking the app instance and config
version associated with the snapshot, as well as any errors that may have
occurred during snapshot operations.

This struct is serialized and stored to ensure its persistence across device
restarts. As a result, special care should be taken when updating the struct
definition in future EVE releases. Fields marked with the `mandatory:"true"` tag
are essential and must be present in all versions of the struct to ensure
backward compatibility.

Defined
in [types/zedmanagertypes.go](../types/zedmanagertypes.go).

### SnapshotDesc

This is a concise representation of a snapshot, containing only the essential
information needed to identify the snapshot and its type. It includes the
SnapshotID and SnapshotType fields. This structure is used when we may not have
all the information required to fully populate the SnapshotInstanceStatus
structure, but we still need to identify the snapshot and its type.

Defined
in [types/zedmanagertypes.go](../types/zedmanagertypes.go).

### VolumesSnapshotConfig

This structure is used to send snapshot requests from the zedmanager to the
volumemgr. It contains fields that specify the snapshot request, such as
SnapshotID, Action, VolumeIDs, and AppUUID. It is created by the zedmanager when
it detects a snapshot request from the controller. The zedmanager uses the
AppInstanceConfig and AppInstanceStatus structures to create a
VolumesSnapshotConfig structure with the appropriate VolumeIDs and Action. This
structure is then sent to the volumemgr to handle the actual snapshot creation,
rollback, or deletion.

Defined
in [types/volumetypes.go](../types/volumetypes.go).

### VolumesSnapshotStatus

This structure is used to send snapshot status from the volumemgr to the
zedmanager. It contains fields that report the status of the snapshot, such as
SnapshotID, VolumeSnapshotMeta, TimeCreated, AppUUID, RefCount, ResultOfAction,
and ErrorAndTimeWithSource. It is used by the volumemgr to report the status of
a snapshot to the zedmanager. The zedmanager uses this information to update the
SnapshottingStatus structure in the AppInstanceStatus structure.

This struct is serialized and stored to ensure its persistence across device
restarts. As a result, special care should be taken when updating the struct
definition in future EVE releases. Fields marked with the `mandatory:"true"` tag
are essential and must be present in all versions of the struct to ensure
backward compatibility.

Defined
in [types/volumetypes.go](../types/volumetypes.go).

## EVE internals of Snapshot Management

### Snapshot Creation Process

The snapshot creation process in EVE involves several steps and interactions
between different agents. Here's a detailed explanation of how the process works
internally.

#### Zedmanager Initial Handling, creation

1. **Detecting a New Snapshot Request**: The process begins in the handleModify
   function in zedmanager, where EVE detects a new request for a snapshot that
   comes with the app configuration from the controller (already parsed by
   zedagent). This is done within the updateSnapshotsInAIStatus function. If a
   new snapshot is detected, EVE sets the state of the application so that the
   next time it will be purged, it will also create a snapshot.
2. **Preparing for Snapshot Creation**: Since a snapshot can only be initiated
   post the application stoppage, and the volumes list within the configuration
   might have already been modified by the time of application termination,
   leading to inconsistencies, EVE prepares the volumesSnapshotConfig with the
   volumes list that was used when the snapshot was requested. This is done in
   the prepareVolumesSnapshotConfigs function.
3. **Storing the App Instance Config**: After preparing the messages to the
   volumemgr, EVE stores the app instance config. This is done in the
   saveAppInstanceConfigForSnapshot function, where the config is serialized and
   stored in a file. This is necessary so that EVE can use it to roll back to
   the previous version if the upgrade fails.
4. **Waiting for App Deactivation**: After the above steps, EVE is ready to
   trigger snapshot creation at a lower level, but it has to wait until the app
   is deactivated. The deactivation of the app happens as a result of the purge
   command handling. Zedmanager "catches" an event of the app deactivation by
   handling a status message from the domain manager. The functions involved in
   this process are handleDomainStatusImpl, updateAIStatusUUID, and
   removeAIStatus.
5. **Triggering Snapshot Creation**: Within the removeAIStatus function,
   zedmanager understands that it is now reacting to domain deactivation. In the
   case where there is a pending snapshot request, zedmanager triggers a message
   to volumemgr. This is done in the triggerSnapshots function, where the time
   triggered for snapshots that are not triggered by time is set, and the
   snapshots are triggered using the list of prepared VolumeSnapshotConfigs. The
   time of snapshot triggering is important later when handling the max snapshot
   limit. If the snapshot is not triggered, the request can be removed, and the
   list of triggered snapshots is sorted based on time.
6. **Publishing the Snapshot Config**: Finally, the volumesSnapshotConfig is
   published, and the prepared volumesSnapshotConfig is removed. The app
   instance status is then published.

#### Voluemmgr Reaction, creation

When volumemgr receives the `volumesSnapshotConfig` from zedmanager, it triggers
a series of actions to handle the snapshot request.
The `handleVolumesSnapshotConfigImpl` function is the entry point for handling
these actions. This function takes the `volumesSnapshotConfig` as input and
performs the appropriate action based on the `Action` field of the config.

1. **Determining the Action**: The `handleVolumesSnapshotConfigImpl` function
   reads the `Action` field of the `volumesSnapshotConfig` to determine what
   action to take. In this case, we are focusing on the `VolumesSnapshotCreate`
   action, which indicates that a snapshot of the specified volumes should be
   created.
2. **Creating the Snapshot**: The `createSnapshot` function is called to handle
   the snapshot creation. It first checks if a snapshot with the specified ID
   already exists. If it does, it logs a warning and returns the existing
   snapshot status. It is possible that a snapshot with the specified ID already
   exists, especially if the system has been restarted and a
   restored `volumesSnapshotConfig` is being handled. If not, it creates a
   new `VolumesSnapshotStatus` with the given snapshot ID and initializes
   the `VolumeSnapshotMeta` map.
3. **Looking Up Volume Status and Getting the Volume Handler**: For each volume
   ID in the VolumeIDs of the `VolumesSnapshotConfig`, the function looks up the
   corresponding volume status. If the volume status is not found, it logs an
   error and returns the snapshot status with the error. The volume handler for
   the volume status is obtained using the `GetVolumeHandler` function. The
   volume handler is an abstraction that helps to call handlers for any
   filesystem-related action specific to a given filesystem. Currently, only the
   handlers (snapshot ones) for ext4 are implemented.
4. **Creating the Volume Snapshot**: The `CreateSnapshot` method of the volume
   handler is called to create a snapshot of the volume. This method returns the
   snapshot metadata, the time the snapshot was created, and any error that
   occurred during the snapshot creation.
5. **Storing Snapshot Metadata**: The snapshot metadata is saved in
   the `VolumeSnapshotMeta` map of the `VolumesSnapshotStatus` with the volume
   ID as the key. The time the snapshot was created is also saved in
   the `VolumesSnapshotStatus`. In the case of the ext4 implementation, the
   snapshot metadata is the snapshot name.
6. **Serializing the Snapshot Status**: The `serializeVolumesSnapshotStatus`
   function is called to serialize the snapshot status to a file. It marshals
   the snapshot status to JSON and writes the JSON data to the snapshot status
   file. This step is crucial for persisting the snapshot information across
   system reboots. Without serialization, the snapshot information would be lost
   from memory upon a system restart, making it impossible to perform operations
   like rollback or deletion of the snapshot after a reboot.
7. **Publishing the Snapshot Status**: After the snapshot has been successfully
   created and the snapshot status has been serialized,
   the `publishVolumesSnapshotStatus` function is called to publish
   the `VolumesSnapshotStatus` object. By publishing the snapshot status,
   volumemgr notifies zedmanager about the result of the snapshot creation
   request. zedmanager can then use this information to update the application's
   status and take any necessary actions based on the snapshot status.

#### Zedmanager Reaction, creation

1. **Receiving the Snapshot Status and Handling Snapshot Creation**: When
   zedmanager receives a `volumesSnapshotStatus` message, it calls
   the `handleVolumesSnapshotStatusImpl` function. The function examines
   the `ResultOfAction` field of the `volumesSnapshotStatus` object to determine
   the type of action that was performed (e.g., snapshot creation, rollback, or
   deletion). If the `ResultOfAction` field indicates that a snapshot was
   created, the `reactToSnapshotCreate` function is called to handle the
   creation of the snapshot.
2. **Looking Up AppInstanceStatus**: The `reactToSnapshotCreate` function begins
   by looking up the `AppInstanceStatus` object associated with the `AppUUID`
   field of the `volumesSnapshotStatus` object. If the `AppInstanceStatus`
   object is not found, an error is logged, and the function returns.
   The `AppInstanceStatus` object is necessary for updating the status of the
   snapshot, which is done in the next step.
3. **Moving Snapshot to Available**: If there is no error, the function logs
   that the snapshot has been created and calls the `moveSnapshotToAvailable`
   function. This function removes the snapshot from the `RequestedSnapshots`
   slice of the `SnapStatus` field of the `AppInstanceStatus` object and adds it
   to the `AvailableSnapshots` slice. The function also updates
   the `TimeCreated` field of the snapshot to the value from
   the `volumesSnapshotStatus` object, marks the snapshot as reported, and
   serializes the `SnapshotInstanceStatus` object to a file.
4. **Publishing AppInstanceStatus**: The `AppInstanceStatus` object is then
   published to reflect the changes made during the snapshot creation process.
5. **Zedagent Handling**: After the publication of the `AppInstanceStatus`, the
   updated status is sent to zedagent. Zedagent then parses the list of
   available snapshots in the `AppInstanceStatus` and forms a reply to the
   controller with the updated snapshot information. This allows the controller
   to be aware of the current state of the snapshots on the device.

### Snapshot Rollback Process

#### Local Application Instance Config

During the rollback process, zedmanager needs to ensure that it uses the correct
version of the app instance config. This is crucial because the rollback is
intended to revert the app instance to a specific state that existed at the time
the snapshot was taken. Using the config just arrived from the controller, which
may not contain the necessary version or fixups, could result in inconsistencies
or errors in the rollback process.

To achieve this, zedmanager uses a local app instance config, which is a copy of
the app instance config that was in use at the time the snapshot was taken. This
local config is stored on the device and has a higher priority than the regular
app instance config received from the controller. Zedmanager always checks the
local config first, ensuring that it uses the correct version of the config
during the rollback process. The local config contains the version of the config
to which the rollback is happening, along with any necessary fixups or
adjustments that may be required for a successful rollback.

The local app instance config is serialized and stored in persistent storage
(`/persist/snapshots` directory). This ensures that the system can handle device
restarts at any point during the snapshot and rollback processes, maintaining
data integrity and operational consistency.

#### Zedmanager Initial Handling, rollback

1. **Detecting a Rollback Request**: The process begins in the `handleModify`
   function in zedmanager, where EVE detects a new request for a snapshot
   rollback by analyzing the `RollbackCmd.Counter` field in the `Snapshot`
   section of the app instance config. If a rollback request is detected, EVE
   sets the state of the application to initiate a rollback to the specified
   snapshot.
2. **Restoring App Instance Config**: EVE restores the app instance config from
   the snapshot by calling the `restoreAppInstanceConfigFromSnapshot` function.
   This function retrieves the snapshot status from the available snapshots and
   restores the app instance config from the snapshot.
   The `deserializeAppInstanceConfigFromSnapshot` function is used to read the
   app instance config from the snapshot file.
3. **Fixing Up App Instance Config**: The `fixupAppInstanceConfig` function is
   called to add fixes to the restored app instance config. The fixes are the
   information that should be taken from the current app instance config, not
   from the snapshot. This function syncs the information about available
   snapshots, restores the restart and purge command counters, and updates the
   app instance config with the fixes. It is important to note that the fixes
   applied here should correspond to the fixes done on the controller side.
4. **Removing Unused Volume Ref Statuses**: The `removeUnusedVolumeRefStatuses`
   function is called to remove the volume ref statuses that are not in the
   snapshot. This is necessary to replace the volumes properly, as the list
   updated in this function is used later by the domain manager to create a VM
   config file.
5. **Switching to Restored Config**: The line  \
   `config = *snappedAppInstanceConfig` \
   indicates that EVE should not use the config received from the controller but
   instead work with the restored config. The `publishLocalAppInstanceConfig`
   function is called to publish the restored config to a dedicated pubsub. This
   pubsub is always checked first by zedmanager to see if there is a local copy
   of the config with higher priority than the one from the controller.
6. **Restarting the Process**: After publishing the local app config,
   the `handleModify` function returns, as the process needs to be restarted
   with the new config. The restart is done from
   the `handleLocalAppInstanceConfigCreate` handler, which is called immediately
   after the local config is published. Now, the `handleModify` function is
   called again with the restored and fixed-up config. This time in
   the `handleModify` function, the condition that previously triggered the
   start of the rollback request handling will be false due to the fixups, and
   the process continues.
7. **Marking the Application for Purge**: Later in the same `handleModify`
   function, EVE checks the `needPurge` flag and the `HasRollbackRequest` flag.
   The `needPurge` flag is returned as true after the `quantifyChanges` function
   due to the fact that the list of volumes has been previously changed (either
   by an explicit purge command or by replacing a volume with a new version as a
   result of an app upgrade). And the `hasRollbackRequst` is set to true on step
   one. So, both flags are true, and EVE marks the application for purge. This is
   necessary because the application needs to be inactive in order to replace
   the volumes, and such volume replacement is technically equivalent to an
   application purge.
8. **Waiting for App Deactivation**: After marking the application for purge,
   zedmanager waits for the application to be deactivated. The detection of the
   application deactivation is done in the `doUpdate` function, which is called
   as part of the process that handles status changes from the domain manager.
9. **Triggering Rollback**: The `triggerRollback` function is called to trigger
   the rollback process. It first checks if there is an
   existing `VolumesSnapshotConfig` in the channel for the active snapshot. If
   found, it updates the action to rollback and publishes the updated config. If
   not found (which can occur if the system has been rebooted), it creates a
   new `VolumesSnapshotConfig` with the necessary information, including the
   snapshot ID, volume IDs, action set to rollback, and app UUID. It then
   publishes the new config to the channel.

#### Volumemgr Reaction, rollback

When volumemgr receives the `volumesSnapshotConfig` from zedmanager, it triggers
a series of actions to handle the snapshot rollback request.
The `handleVolumesSnapshotConfigImpl` function is the entry point for handling
these actions. This function takes the `volumesSnapshotConfig` as input and
performs the appropriate action based on the Action field of the config.

1. **Determining the Action**: The `handleVolumesSnapshotConfigImpl` function
   reads the `Action` field of the `volumesSnapshotConfig` to determine what
   action to take. In this case, we are focusing on
   the `VolumesSnapshotRollback` action, which indicates that a rollback to the
   specified snapshot should be performed.
2. **Validating Volumes Snapshot Status**: The `rollbackSnapshot` function is
   called to handle the snapshot rollback. It first checks if
   a `volumesSnapshotStatus` exists for the given SnapshotID. If not, a new
   volumesSnapshotStatus is created to report an error. The status contains the
   necessary metadata for the following steps.
3. **Getting the Volume Handler**: For each volume in the `VolumeSnapshotMeta`
   of the `volumesSnapshotStatus`, the function retrieves the
   corresponding `volumeStatus` using the `lookupVolumeStatusByUUID` function.
   If the `volumeStatus` is not found, an error is reported in
   the `volumesSnapshotStatus`. The volume status is used to get the proper
   volume handler that keeps the low-level FS-specific handlers for the volume
   operations.
4. **Rolling Back the Volume Snapshot**: The `RollbackToSnapshot` method of the
   volume handler is called with the snapshot metadata (`snapMeta`) to perform
   the rollback. The outcome of the rollback (success or error) is recorded in
   the `volumesSnapshotStatus`.
5. **Publishing the Snapshot Status**: After the rollback has been successfully
   performed or an error has occurred, the `publishVolumesSnapshotStatus`
   function is called to publish the `VolumesSnapshotStatus` object. By
   publishing the snapshot status, volumemgr notifies zedmanager about the
   result of the snapshot rollback request. zedmanager can then use this
   information to update the application's status and take any necessary actions
   based on the snapshot status.

#### Zedmanager Reaction, rollback

1. **Receiving the Snapshot Status and Handling Snapshot Rollback**: When
   zedmanager receives a `volumesSnapshotStatus` message, it calls
   the `handleVolumesSnapshotStatusImpl` function. The function examines
   the `ResultOfAction` field of the `volumesSnapshotStatus` object to determine
   the type of action that was performed (e.g., snapshot creation, rollback, or
   deletion). If the `ResultOfAction` field indicates that a snapshot rollback
   was performed, the `reactToSnapshotRollback` function is called to handle the
   rollback of the snapshot.
2. **Looking Up AppInstanceStatus**: The `reactToSnapshotRollback` function
   begins by looking up the `AppInstanceStatus` object associated with
   the `AppUUID` field of the `volumesSnapshotStatus` object. If
   the `AppInstanceStatus` object is not found, an error is logged, and the
   function returns. The `AppInstanceStatus` object is necessary for updating
   the status of the snapshot, which is done in the next step.
3. **Handling Errors**: If the `volumesSnapshotStatus` object has an error (as
   indicated by the `HasError` function), the function logs the error and sets
   the `Error`, `ErrorTime`, and `ErrorDescription` fields of
   the `AppInstanceStatus` object. The `setSnapshotStatusError` function is
   called to update the error status of the snapshot. The `AppInstanceStatus`
   object is then published to reflect the error status.
4. **Rollback Completion**: If there is no error, the function sets
   the `RollbackInProgress` field of the `SnapStatus` field of
   the `AppInstanceStatus` object to false, indicating that the rollback is
   complete. The function then unpublishes the local app instance config, as
   once the rollback is complete, it’s no longer needed. The app instance has
   been successfully reverted to the state it was in at the time of the
   snapshot, and any subsequent operations should use the latest app instance
   config received from the controller. Therefore, zedmanager unpublishes the
   local app instance config, removing it from consideration in future
   operations. The `AppInstanceStatus` object is then published to reflect the
   changes made during the rollback process. The local
5. **Zedagent Handling**: After the publication of the AppInstanceStatus, the
   updated status is sent to zedagent. Zedagent then parses the list of
   available snapshots in the AppInstanceStatus and forms a reply to the
   controller with the updated snapshot information. This allows the controller
   to be aware of the current state of the snapshots on the device.

#### Reporting Successful Rollback to the Controller

Successful rollback is reported back to the controller in an app instance status
that includes the version of the app instance config that corresponds to the
snapshotted config version used during the rollback.

The process of setting up the version in the app instance status happens in
the `doUpdate` function. This function is responsible for updating the app
instance status with the appropriate version information that matches the
snapshotted config version used in the rollback.

Once the version is set in the app instance status, zedmanager sends the updated
app instance status back to the controller. The very first app instance status
report sent to the controller that contains the version corresponding to the
snapshotted config version serves as an indication of a successful rollback. The
controller receives this report and becomes aware of the successful rollback and
the current state of the app instance on the device.

### Snapshot Deletion Process

#### Zedmanager Initial Handling, deletion

1. **Identifying Snapshots for Deletion**: In the `handleModify` function,
   zedmanager detects that a snapshot is requested to be deleted by calling
   the `updateSnapshotsInAIStatus` function. The
   function `getSnapshotsToBeDeleted` is called
   within `updateSnapshotsInAIStatus` to identify the snapshots that need to be
   deleted. It returns a list of snapshots to be deleted based on the snapshots
   reported to the controller and the snapshots present in the configuration.
2. **Handling Excess Snapshot Requests**: The function `adjustToMaxSnapshots` is
   called to verify if the number of snapshots exceeds the set limit. If it
   does, it marks the oldest snapshots for deletion.
3. **Setting Snapshots for Deletion**: The function `updateSnapshotsInAIStatus`
   sets the `SnapshotsToBeDeleted` field in the `SnapStatus` of
   the `AppInstanceStatus` with the list of snapshots identified for deletion.
4. **Immediate Deletion Reporting**: The function `updateSnapshotsInAIStatus`
   removes the snapshots marked for deletion from the `AvailableSnapshots` list
   in the `SnapStatus` of the `AppInstanceStatus`. Due to the current
   implementation limitation on the controller side, the device reports
   successful deletion immediately back to the controller, even though the
   deletion may be deferred.

5. **Wait For Deferred Deactivation**
   The steps described above can be considered as preparation for deletion. Even
   though snapshot removal requires VM deactivation, zedmanager does not trigger
   it. EVE has to wait for the next app deactivation to proceed with the
   deletion. Therefore, there is no immediate messaging to volumemgr, and the
   actual deletion is deferred until the next app deactivation.

    1. **Regular App Reactivation**: In the `doUpdate` function, zedmanager
       checks if the VM is already shut down. If it is, and there are snapshots
       marked for deletion, it triggers the snapshot deletion process. The
       function is part of statuses from the domain manager handling. A new
       status comes when the domain manager deactivates the application.
    2. **Handling of Purge**: In the removeAIStatus function, zedmanager handles
       the case where an app is deactivated due to a purge command. If the VM is
       shut down and there are snapshots marked for deletion, it triggers the
       snapshot deletion process first. This is done to ensure compliance with
       the maximum limit of snapshots. Only after the deletion process is
       triggered, if a snapshot is requested on upgrade and there are prepared
       volume snapshot configs, zedmanager triggers the creation of new
       snapshots.

   In both cases, the `triggerSnapshotDeletion` function is called.

6. **Unpublishing Message to Volumemgr**: In the `triggerSnapshotDeletion`
   function, for each snapshot to be deleted, zedmanager looks up the
   corresponding `VolumesSnapshotConfig`. If the snapshot has already been
   triggered, zedmanager sets the action to VolumesSnapshotDelete and
   unpublishes the VolumesSnapshotConfig to notify volumemgr about the deletion
   request.

#### Volumemgr Reaction, deletion

1. **Handling and Processing the Deletion Request**: When a snapshot deletion
   request is received, the `handleVolumesSnapshotConfigDelete` function is
   called. This function sets the action to `VolumesSnapshotDelete` and calls
   the `handleVolumesSnapshotConfigImpl` function to process the snapshot
   deletion request. Setting the action is necessary as the message on
   unpublishing reflects the last state when it was published, so it's not
   possible to update it on the caller (zedmanager) side.
2. **Deleting the Snapshot**: In the `handleVolumesSnapshotConfigImpl` function,
   the `deleteSnapshot` function is called to handle the snapshot deletion based
   on the action specified in the config.
3. **Retrieving Snapshot Status and Metadata**: The `deleteSnapshot` function
   looks up the `VolumesSnapshotStatus` for the snapshot to be deleted. This
   status is necessary to get the metadata and the file system specific handlers
   for the snapshot. If the status is not found, it creates a
   new `VolumesSnapshotStatus` to report an error. Otherwise, it proceeds with
   the deletion process.
4. **Iterating Over Volumes and Retrieving Volume Handlers**: The function
   iterates over the volumes associated with the snapshot, as specified in
   the `VolumeSnapshotMeta` field of the `VolumesSnapshotStatus`. For each
   volume, it looks up the corresponding `VolumeStatus` and retrieves the
   appropriate volume handler. The volume handler is responsible for performing
   filesystem-specific operations on the volume, such as creating, deleting, or
   rolling back snapshots.
5. **Deleting the Snapshot for Each Volume**:  The function calls
   the `DeleteSnapshot` method of the volume handler to delete the snapshot for
   the specified volume according to the low-level logic. It does not touch the
   files created explicitly by EVE; they will be deleted later by _zedmanager_.
   If an error occurs during the deletion process, it sets an error in the
   VolumesSnapshotStatus.
6. **Unpublishing the Snapshot Status**: After the snapshot has been deleted for
   all volumes, the `unpublishVolumesSnapshotStatus` function is called to
   unpublish the `VolumesSnapshotStatus` for the deleted snapshot. This is
   necessary to notify zedmanager about the result of the deletion.

#### Zedmanager Reaction, deletion

1. **Handling Snapshot Deletion Status**: When a snapshot deletion status is
   received, the `handleVolumesSnapshotStatusDelete` function is called. This
   function sets the `ResultOfAction` field of the `volumesSnapshotStatus`
   to `VolumesSnapshotDelete` and calls the `handleVolumesSnapshotStatusImpl`
   function to handle the snapshot deletion status.
2. **Processing the Deletion Status**: The `handleVolumesSnapshotStatusImpl`
   function processes the snapshot deletion status based on the `ResultOfAction`
   field of the `volumesSnapshotStatus`. In the case of snapshot deletion, it
   calls the `reactToSnapshotDelete` function.
3. **Reacting to Snapshot Deletion**: The `reactToSnapshotDelete` function looks
   up the `AppInstanceStatus` for the app associated with the snapshot. If the
   status is not found, it logs an error and returns. If
   the `volumesSnapshotStatus` has an error, it sets the error in
   the `AppInstanceStatus` and calls the `setSnapshotStatusError` function to
   set the error for the specific snapshot. It then publishes the
   updated `AppInstanceStatus`.
4. **Deleting the Snapshot from App Status**: If there is no error, the function
   calls the `deleteSnapshotFromStatus` function to delete the snapshot from
   the `AppInstanceStatus`. This function removes the snapshot from the list of
   requested snapshots, available snapshots, and snapshots to be triggered. It
   also removes the prepared volumes snapshot config for the snapshot.
5. **Deleting Snapshot Files**: The `reactToSnapshotDelete` function calls
   the `DeleteSnapshotFiles` function to delete the snapshot directory. This
   function checks if the snapshot directory exists and, if it does, removes the
   directory. It's important to note that this step only removes the files that
   were explicitly created by EVE. It does not remove the files created by the
   filesystem-specific handlers, as those files are managed separately and have
   already been removed by volumemgr.
6. **Publishing App Instance Status**: After deleting the snapshot from the app
   status and deleting the snapshot files, the `reactToSnapshotDelete` function
   publishes the updated AppInstanceStatus.

## Low-Level Snapshot Management for ext4

In EVE, managing snapshots for ext4 file systems involves interactions with the
application lifecycle and low-level operations using the `qemu-img` tool. Due to
the nature of disk image snapshots, certain operations can only be performed
when the application is not actively using the volume. This section explains how
EVE handles snapshot operations in conjunction with the application lifecycle
and the low-level details of snapshot management.

### Snapshot Creation for ext4

When creating a new snapshot, EVE must stop the application to ensure that the
volume is not in use. The need to stop the application arises from the fact that
the snapshot request is now triggered during the handling of a purge command,
and an application reboot is part of this handling process. After stopping the
application, EVE performs the snapshot operation using the qemu-img tool with
the command

```bash
qemu-img snapshot -c snapshot_name /path/to/base_image.qcow2
```

The tool creates the snapshot implicitly, and the associated files are not
exposed to EVE. EVE operates only with the snapshot IDs and stores them as
metadata. Once the snapshot operation is complete, EVE explicitly reactivates
the application. This process ensures that the snapshot is created in a
consistent and safe manner.

### Snapshot Rollback for ext4

Similar to snapshot creation, performing a rollback to a previous snapshot
requires stopping the application. But in this case, EVE triggers it explicitly.
EVE uses the qemu-img tool with the command

```bash
qemu-img snapshot -a snapshot_name /path/to/base_image.qcow2
```

to apply the snapshot. After the rollback operation is complete, EVE reactivates
the application.

### Snapshot Deletion for ext4

Unlike snapshot creation and rollback, snapshot deletion does not require an
immediate application reboot. When EVE receives a command to delete a snapshot,
it marks the snapshot as "to be deleted" in its internal metadata. However, the
actual deletion of the snapshot and the associated files is deferred until the
next application reboot. This approach avoids disrupting the application's
operation for snapshot deletion. EVE uses the qemu-img tool with the command

```bash
qemu-img snapshot -d snapshot_name /path/to/base_image.qcow2
```

to delete the snapshot. Despite the deferred deletion, EVE immediately reports
back to the controller that the snapshot has been removed. This is done to keep
the controller informed and prevent it from reporting snapshots that no longer
exist according to its records. This approach ensures that the controller's view
of the snapshots is consistent with EVE's internal state.

EVE manages snapshots for ext4 file systems by coordinating snapshot operations
with the application lifecycle and performing low-level snapshot operations
using the qemu-img tool. Snapshot creation and rollback require stopping and
reactivating the application to ensure data consistency. Snapshot deletion is
deferred until the next application reboot but is immediately reported to the
controller to maintain consistency in the system's view of the snapshots. This
approach allows EVE to efficiently manage snapshots while minimizing disruptions
to the application's operation.

## Low-Level Snapshot Management for ZFS

In the EVE project, managing snapshots for ZFS file systems is conducted through
direct interaction with the ZFS filesystem via the libzfs library. This approach
enables EVE to perform snapshot operations such as creation, rollback, and
deletion with high efficiency and reliability. Despite the fact that ZFS
supports creation and usage of snapshots while the file system is in use, we
still want to wait for the application to be inactive before performing any
snapshot operations. This is done to make the code consistent with the ext4 file
based snapshots and to avoid any potential issues that may arise from the
application's active use of the volume.

### Snapshot Creation for ZFS

For creating snapshots, EVE employs the `libzfs.DatasetSnapshot` function. This
function is instrumental in generating a snapshot of the specified ZFS volume.
The process begins with EVE generating a unique, timestamped snapshot name to
ensure distinguishability. The DatasetSnapshot function is then called with the
dataset name and the generated snapshot name. The operation's success,
including the snapshot name and timestamp, is logged for transparency and
auditability.

### Snapshot Rollback for ZFS

To roll a ZFS volume back to a previous state, EVE uses the
`libzfs.DatasetRollback` function. EVE initiates the rollback by first
identifying the specific snapshot and dataset targeted for rollback. The
`DatasetRollback` function in libzfs takes the dataset object and the snapshot
object as arguments. The process is thoroughly logged, indicating the dataset
and snapshot involved,  to ensure the operation's success is recorded.

### Snapshot Deletion for ZFS

For snapshot deletion, EVE leverages the `libzfs.DestroyDataset` function. This
step involves validating the snapshot's existence and association with the
correct volume before proceeding with deletion. Once validated, the
`DestroyDataset` function is called with the name of the snapshot to be deleted.
The deletion operation is also logged, providing a record of the action for
auditing purposes.

## Copy-on-Write (CoW) Approach

Both the qemu-img tool and ZFS utilize the Copy-on-Write (CoW) approach for
efficient snapshot management. This technique is foundational in ensuring
snapshots are created without requiring immediate additional storage space
proportional to the dataset or disk image size.

In the CoW model, when a snapshot is initiated, the system does not duplicate
the entire file system or disk image. Instead, it maintains references to the
existing data blocks. As modifications occur, new data is written to different
locations, preserving the original data blocks as they were at the snapshot's
creation time. This mechanism ensures that snapshots initially have a minimal
storage footprint, only increasing as changes accrue in the file system or disk
image over time.

## Handling Purgeable Volumes in EVE

Purgeable volumes in EVE are unique in that they can be entirely replaced with
new versions during the execution of a purge command. This behavior requires a
different approach to snapshot management compared to non-purgeable volumes.

### Snapshot Creation for Purgeable Volumes

When a snapshot is created for a purgeable volume, EVE retains the entire old
volume as a backup. This backup includes all the data in the volume at the time
of the snapshot. Unlike non-purgeable volumes, where a delta file is typically
created, purgeable volumes require the entire volume to be preserved. During the
snapshot creation, EVE retains the original volume file in its existing
location, ensuring that the old volume is preserved in its entirety.

### Rollback Operation for Purgable Volumes

In the event of a rollback to a previous snapshot, EVE restores the purgeable
volume by using the original volume file that was retained during the snapshot
creation. The process involves updating the volume ID in the domain config to
point to the original volume file, effectively restoring the volume to its state
at the time of the snapshot.

### Snapshot Deletion for Purgable Volumes

When a snapshot of a purgeable volume is deleted, EVE removes any references to
the original volume file associated with the snapshot ID.

## Serialization and Deserialization

The snapshotting feature relies on robust serialization and deserialization
functionality to ensure the continuity of snapshot lists after the system
reboots. The serialization process converts snapshot data structures into a
format that can be stored on disk, while deserialization restores the data
structures from the stored format.

Key Points:

* Serialization and deserialization functions have been implemented
  for `VolumesSnapshotStatus`, and `SnapshotInstanceStatus`.
* Comprehensive field checks are performed during deserialization to ensure data
  integrity.
* Critical fields essential for snapshot restoration are checked during
  deserialization.
* The `AppInstanceConfig` is stored using a simple conversion to JSON format
  without checking extra fields. This storage is necessary not only for system
  reboots but also for snapshotting the configuration.

The inclusion of AppInstanceConfig storage is important as it ensures that the
application's configuration is preserved and can be restored as part of the
snapshotting process.

### Mandatory Tag in Struct Fields

In the deserialization process, the `extractFields` function inspects the
struct's fields and checks for a mandatory tag. If a field
has `mandatory="true"`, it is considered a critical field. The absence of any
critical field in the JSON file will result in an error during the
deserialization process.

Consider the `SnapshotInstanceStatus` struct:

```go
type SnapshotInstanceStatus struct {
  Snapshot      SnapshotDesc `mandatory:"true"`
  Reported      bool
  TimeTriggered time.Time
  TimeCreated   time.Time
  AppInstanceID uuid.UUID `mandatory:"true"`
  ConfigVersion UUIDandVersion `mandatory:"true"`
  Error         ErrorDescription
}
```

In this struct, the fields `Snapshot`, `AppInstanceID`, and `ConfigVersion` are
marked as mandatory with `mandatory:"true"`. During deserialization, if any of
these fields are missing in the JSON file, an error will be triggered. This
ensures that all essential data is present before the deserialization process
completes, thereby maintaining data integrity.

The mandatory tag serves a critical role in ensuring forward and backward
compatibility between different versions of EVE. When a field in a struct is
tagged as mandatory, it signifies that this field must be present in all
serialized instances of the struct, across all versions of EVE. This is crucial
for maintaining compatibility and ensuring that deserialization will succeed
even if the struct has been modified in a newer version of EVE.

The serialized snapshot data is stored in the `/persist/snapshots` directory on
the device. This ensures that the snapshot information is preserved across
system reboots and can be easily accessed for deserialization and further
processing. The constants that define the names of the files and the directory
are located in the
[types/locationconsts.go](../types/locationconsts.go)
file.

## Ensuring Maximum Snapshots Limit

EVE ensures that the number of snapshots does not exceed the maximum limit set
by the controller. This is achieved through the `adjustToMaxSnapshots` function,
which verifies the total number of snapshots and marks the oldest snapshots for
deletion if the limit is exceeded. The function also trims the list of upcoming
snapshots to fit within the limit. The order in which the lists are considered
for deletion is as follows:

1. Existing snapshots
2. New snapshot requests
3. Snapshots prepared for capture

The function works as follows:

1. **Calculate Total Snapshots**: The function calculates the total number of
   snapshots requested by adding the number of available snapshots, requested
   snapshots, and new requested snapshots, and subtracting the number of
   snapshots to be deleted.
2. **Check Limit**: If the total number of snapshots is less than or equal to
   the maximum limit, no action is needed.
3. **Delete Oldest Snapshots**: The function sorts the available snapshots by
   creation time and flags the oldest snapshots for deletion until the total
   number of snapshots is within the limit.
4. **Delete from Requests List**: If there are still snapshots to be deleted,
   the function removes them from the new snapshot requests list.
5. **Delete Unpublished Snapshots**: If there are still snapshots to be deleted,
   the function flags planned but not yet published snapshots (for which EVE has
   prepared volumes snapshot config, but it’s not sent to _volumemgr_ yet) for
   deletion.
6. **Return Updated Lists**: The function returns the updated lists of snapshots
   to be deleted and new requested snapshots.

It's important to note that EVE triggers snapshot deletions (possibly requested
as a result of this adjustment) before snapshot creation to ensure compliance
with the maximum limit.

Currently, the controller only supports a maximum of 1 snapshot, so this
functionality is not extensively tested. However, it is designed to handle
scenarios where the maximum limit is set to a higher value.

## Error Handling

The snapshotting feature includes comprehensive error handling to ensure
robustness and reliability. Key points:

* Errors are set and propagated to the status sent to the controller.
* Error handling includes checking for missing critical fields during
  deserialization.
* Errors during rollback are reported with additional details such as time and
  severity.

## Testing

The snapshotting feature has been thoroughly tested to ensure its robustness and
reliability. It contains several types of tests.

### Unit Tests

A suite of unit tests has been introduced for the `DeserializeToStruct` function
in the utils package. The tests cover various scenarios, including simple
struct, error conditions, mandatory field logic, extra field in file, missing
non-mandatory field in file, struct with anonymous field, nested structs, array
of structs, and anonymous structs.

### Integration Tests

The snapshot integration testing in EVE is currently performed manually. Each
test case is executed step-by-step by a tester to verify the functionality of
the snapshot feature. Automation of the test cases would streamline the testing
process and enable more frequent and consistent testing.

#### Environment

The testing environment is set up with a KVM-based VM as the application
instance, which has two attached volumes: both are encrypted HDD disks, one is
purgeable, and the other is not. The EVE build used for testing supports ext4
volumes (no ZFS).

#### Test Cases

The test cases cover a wide range of scenarios, including:

* **Snapshot Creation**: Verifying the successful creation of snapshots.
* **Rollback to Snapshot**: Testing the rollback functionality to a previously
  created snapshot, including scenarios with non-purgeable volumes and purgeable
  volumes.
* **Delete Snapshot**: Testing the deletion of a snapshot under different
  conditions, such as without restarting the system and after a system restart.
* **System Restart Impact**: Assessing the impact of a system restart on
  snapshot creation and rollback.
* **Create a New Snapshot**: Testing the creation of new snapshots under various
  conditions, including when a previous snapshot has been deleted or still
  exists.
* **Node Deletion**: Testing the impact of deleting a node with a VM that had a
  snapshot.
* **App Instance Deletion**: Testing the deletion of an application instance
  after a rollback and verifying cleanups.
