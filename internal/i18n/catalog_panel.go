package i18n

// Stable keys for the HTTPS panel. The panel keeps English strings in its
// templates and Go code as the source language and translates them through the
// same Translator as the launcher.
const (
	KeyPanelNavAdminSections = "panel.nav.admin_sections"
	KeyPanelNavDashboard     = "panel.nav.dashboard"
	KeyPanelNavLogs          = "panel.nav.logs"
	KeyPanelNavMods          = "panel.nav.mods"
	KeyPanelNavState         = "panel.nav.state"
	KeyPanelNavConfig        = "panel.nav.config"
	KeyPanelNavStaff         = "panel.nav.staff"
	KeyPanelNavPublicArchive = "panel.nav.public_archives"
	KeyPanelNavAccount       = "panel.nav.account"
	KeyPanelNavPassword      = "panel.nav.change_password"
	KeyPanelNavLogout        = "panel.nav.logout"
	KeyPanelNavPublicPages   = "panel.nav.public_pages"
	KeyPanelNavHome          = "panel.nav.home"
	KeyPanelNavArchives      = "panel.nav.archives"
	KeyPanelNavLogin         = "panel.nav.login"
	KeyPanelNavLanguage      = "panel.nav.language"

	KeyPanelNoticeRestartRequired = "panel.notice.restart_required"

	KeyPanelLoginTitle    = "panel.login.title"
	KeyPanelLoginEyebrow  = "panel.login.eyebrow"
	KeyPanelLoginHeading  = "panel.login.heading"
	KeyPanelLoginHint     = "panel.login.hint"
	KeyPanelLoginPassword = "panel.login.password"
	KeyPanelLoginSubmit   = "panel.login.submit"
	KeyPanelLoginBack     = "panel.login.back"

	KeyPanelPasswordTitle      = "panel.password.title"
	KeyPanelPasswordEyebrow    = "panel.password.eyebrow"
	KeyPanelPasswordHeading    = "panel.password.heading"
	KeyPanelPasswordMustChange = "panel.password.must_change"
	KeyPanelPasswordCurrent    = "panel.password.current"
	KeyPanelPasswordNew        = "panel.password.new"
	KeyPanelPasswordConfirm    = "panel.password.confirm"
	KeyPanelPasswordSubmit     = "panel.password.submit"
	KeyPanelPasswordBack       = "panel.password.back"

	KeyPanelHomeEyebrow        = "panel.home.eyebrow"
	KeyPanelHomeConnect        = "panel.home.connect"
	KeyPanelHomePort           = "panel.home.port"
	KeyPanelHomeStatusCaption  = "panel.home.status_caption"
	KeyPanelHomeArchivesCTA    = "panel.home.archives_cta"
	KeyPanelHomeConnection     = "panel.home.connection"
	KeyPanelHomeAddress        = "panel.home.address"
	KeyPanelHomePortLabel      = "panel.home.port_label"
	KeyPanelHomeStatus         = "panel.home.status"
	KeyPanelHomePublicArchives = "panel.home.public_archives"
	KeyPanelHomePublicHint     = "panel.home.public_hint"
	KeyPanelHomeSnapshots      = "panel.home.snapshots"
	KeyPanelHomeMods           = "panel.home.mods"

	KeyPanelBackupsTitle           = "panel.backups.title"
	KeyPanelBackupsEyebrow         = "panel.backups.eyebrow"
	KeyPanelBackupsHeading         = "panel.backups.heading"
	KeyPanelBackupsLead            = "panel.backups.lead"
	KeyPanelBackupsStateHeading    = "panel.backups.state_heading"
	KeyPanelBackupsStateHint       = "panel.backups.state_hint"
	KeyPanelBackupsModsHeading     = "panel.backups.mods_heading"
	KeyPanelBackupsModsHint        = "panel.backups.mods_hint"
	KeyPanelBackupsStateEmpty      = "panel.backups.state_empty"
	KeyPanelBackupsModsEmpty       = "panel.backups.mods_empty"
	KeyPanelBackupsCurrent         = "panel.backups.current"
	KeyPanelBackupsModsLabel       = "panel.backups.mods_label"
	KeyPanelBackupsModsFileMissing = "panel.backups.mods_file_missing"
	KeyPanelBackupsLinkUnreadable  = "panel.backups.link_unreadable"
	KeyPanelBackupsDownloadState   = "panel.backups.download_state"
	KeyPanelBackupsDownloadBundle  = "panel.backups.download_bundle"
	KeyPanelBackupsDownloadMods    = "panel.backups.download_mods"
	KeyPanelBackupsOlderState      = "panel.backups.older_state"
	KeyPanelBackupsOlderMods       = "panel.backups.older_mods"
	KeyPanelBackupsLimit           = "panel.backups.limit"
	KeyPanelBackupsShow            = "panel.backups.show"

	KeyPanelCommonArchive       = "panel.common.archive"
	KeyPanelCommonSize          = "panel.common.size"
	KeyPanelCommonDateUTC       = "panel.common.date_utc"
	KeyPanelCommonActions       = "panel.common.actions"
	KeyPanelCommonAction        = "panel.common.action"
	KeyPanelCommonShow          = "panel.common.show"
	KeyPanelCommonCurrent       = "panel.common.current"
	KeyPanelCommonPause         = "panel.common.pause"
	KeyPanelCommonResume        = "panel.common.resume"
	KeyPanelCommonChangedPrefix = "panel.common.changed_prefix"
	KeyPanelCommonNoChanges     = "panel.common.no_changes"
)

const (
	KeyPanelDashboardTitle          = "panel.dashboard.title"
	KeyPanelDashboardEyebrow        = "panel.dashboard.eyebrow"
	KeyPanelDashboardLead           = "panel.dashboard.lead"
	KeyPanelDashboardServer         = "panel.dashboard.server"
	KeyPanelDashboardSince          = "panel.dashboard.since"
	KeyPanelDashboardAddress        = "panel.dashboard.address"
	KeyPanelDashboardHealth         = "panel.dashboard.health"
	KeyPanelDashboardMods           = "panel.dashboard.mods"
	KeyPanelDashboardStart          = "panel.dashboard.start"
	KeyPanelDashboardRestart        = "panel.dashboard.restart"
	KeyPanelDashboardStop           = "panel.dashboard.stop"
	KeyPanelDashboardLogs           = "panel.dashboard.logs"
	KeyPanelDashboardConfirmRestart = "panel.dashboard.confirm_restart"
	KeyPanelDashboardConfirmStop    = "panel.dashboard.confirm_stop"
	KeyPanelDashboardDoctorHint     = "panel.dashboard.doctor_hint"
	KeyPanelDashboardResources      = "panel.dashboard.resources"
	KeyPanelDashboardWindowAria     = "panel.dashboard.window_aria"
	KeyPanelDashboardWindow1h       = "panel.dashboard.window_1h"
	KeyPanelDashboardWindow6h       = "panel.dashboard.window_6h"
	KeyPanelDashboardWindow24h      = "panel.dashboard.window_24h"
	KeyPanelDashboardRAM            = "panel.dashboard.ram"
	KeyPanelDashboardGameCPU        = "panel.dashboard.game_cpu"
	KeyPanelDashboardSysCPU         = "panel.dashboard.sys_cpu"
	KeyPanelDashboardSysRAM         = "panel.dashboard.sys_ram"
	KeyPanelDashboardSwap           = "panel.dashboard.swap"
	KeyPanelDashboardDiskFree       = "panel.dashboard.disk_free"
	KeyPanelDashboardNoCgroup       = "panel.dashboard.no_cgroup"
	KeyPanelDashboardRAMChart       = "panel.dashboard.ram_chart"
	KeyPanelDashboardRAMHint        = "panel.dashboard.ram_hint"
	KeyPanelDashboardCPUChart       = "panel.dashboard.cpu_chart"
	KeyPanelDashboardCPUHint        = "panel.dashboard.cpu_hint"
	KeyPanelDashboardDiskChart      = "panel.dashboard.disk_chart"
	KeyPanelDashboardDiskHint       = "panel.dashboard.disk_hint"
	KeyPanelDashboardNoScript       = "panel.dashboard.noscript"
	KeyPanelDashboardEvents         = "panel.dashboard.events"
	KeyPanelDashboardEventsEmpty    = "panel.dashboard.events_empty"
	KeyPanelDashboardOpenLogs       = "panel.dashboard.open_logs"
	KeyPanelDashboardAuditHint      = "panel.dashboard.audit_hint"
	KeyPanelDashboardQuickActions   = "panel.dashboard.quick_actions"
	KeyPanelDashboardCreateState    = "panel.dashboard.create_state"
	KeyPanelDashboardUploadMods     = "panel.dashboard.upload_mods"
	KeyPanelDashboardRestore        = "panel.dashboard.restore"

	KeyPanelStateTitle          = "panel.state.title"
	KeyPanelStateEyebrow        = "panel.state.eyebrow"
	KeyPanelStateHeading        = "panel.state.heading"
	KeyPanelStateLead           = "panel.state.lead"
	KeyPanelStateMissingMods    = "panel.state.missing_mods"
	KeyPanelStateCurrentMods    = "panel.state.current_mods"
	KeyPanelStateRestoreHeading = "panel.state.restore_heading"
	KeyPanelStateRestoreHint    = "panel.state.restore_hint"
	KeyPanelStateRestoreConfirm = "panel.state.restore_confirm"
	KeyPanelStateFileLabel      = "panel.state.file_label"
	KeyPanelStateUploadRestore  = "panel.state.upload_restore"
	KeyPanelStateWhatChanges    = "panel.state.what_changes"
	KeyPanelStateWhatChangesTxt = "panel.state.what_changes_text"
	KeyPanelStateCreate         = "panel.state.create"
	KeyPanelStateManageMods     = "panel.state.manage_mods"
	KeyPanelStateLimit          = "panel.state.limit"
	KeyPanelStateEmpty          = "panel.state.empty"
	KeyPanelStateMods           = "panel.state.mods"
	KeyPanelStateModsMissing    = "panel.state.mods_missing"
	KeyPanelStateModsActive     = "panel.state.mods_active"
	KeyPanelStateModsInactive   = "panel.state.mods_inactive"
	KeyPanelStateDownloadState  = "panel.state.download_state"
	KeyPanelStateDownloadMods   = "panel.state.download_mods"
	KeyPanelStateDeployMods     = "panel.state.deploy_mods"
	KeyPanelStateDeployConfirm  = "panel.state.deploy_confirm"
	KeyPanelStateRestoreButton  = "panel.state.restore_button"
	KeyPanelStateDeployFirst    = "panel.state.deploy_first"
	KeyPanelStateOlder          = "panel.state.older"
)

const (
	KeyPanelModsTitle           = "panel.mods.title"
	KeyPanelModsEyebrow         = "panel.mods.eyebrow"
	KeyPanelModsHeading         = "panel.mods.heading"
	KeyPanelModsLead            = "panel.mods.lead"
	KeyPanelModsCurrentHeading  = "panel.mods.current_heading"
	KeyPanelModsNotDeployed     = "panel.mods.not_deployed"
	KeyPanelModsUploadHeading   = "panel.mods.upload_heading"
	KeyPanelModsUploadHint      = "panel.mods.upload_hint"
	KeyPanelModsFormat          = "panel.mods.format"
	KeyPanelModsFormatRoot      = "panel.mods.format_root"
	KeyPanelModsFormatPlugins   = "panel.mods.format_plugins"
	KeyPanelModsFormatCRC       = "panel.mods.format_crc"
	KeyPanelModsFormatBaseESM   = "panel.mods.format_base_esm"
	KeyPanelModsFormatScripts   = "panel.mods.format_scripts"
	KeyPanelModsExample         = "panel.mods.example"
	KeyPanelModsFileLabel       = "panel.mods.file_label"
	KeyPanelModsUpload          = "panel.mods.upload"
	KeyPanelModsArchivesHeading = "panel.mods.archives_heading"
	KeyPanelModsEmpty           = "panel.mods.empty"
	KeyPanelModsCurrentTag      = "panel.mods.current_tag"
	KeyPanelModsDeploy          = "panel.mods.deploy"
	KeyPanelModsRollbackHeading = "panel.mods.rollback_heading"
	KeyPanelModsRollbackText    = "panel.mods.rollback_text"
	KeyPanelModsRollback        = "panel.mods.rollback"

	KeyPanelConfigTitle      = "panel.config.title"
	KeyPanelConfigEyebrow    = "panel.config.eyebrow"
	KeyPanelConfigHeading    = "panel.config.heading"
	KeyPanelConfigLead       = "panel.config.lead"
	KeyPanelConfigHint       = "panel.config.hint"
	KeyPanelConfigFieldError = "panel.config.field_error"
	KeyPanelConfigYes        = "panel.config.yes"
	KeyPanelConfigNo         = "panel.config.no"
	KeyPanelConfigSave       = "panel.config.save"
	KeyPanelConfigReset      = "panel.config.reset"
	KeyPanelConfigStatusIdle = "panel.config.status_idle"

	KeyPanelConfigSectionServer  = "panel.config.section_server"
	KeyPanelConfigSectionGame    = "panel.config.section_game"
	KeyPanelConfigSectionBanlist = "panel.config.section_banlist"
	KeyPanelConfigGameNote       = "panel.config.game_note"
	KeyPanelConfigHostname       = "panel.config.field_hostname"
	KeyPanelConfigPort           = "panel.config.field_port"
	KeyPanelConfigMaxPlayers     = "panel.config.field_max_players"
	KeyPanelConfigPassword       = "panel.config.field_password"
	KeyPanelConfigLogLevel       = "panel.config.field_log_level"
	KeyPanelConfigBanNames       = "panel.config.banlist_names"
	KeyPanelConfigBanIPs         = "panel.config.banlist_ips"

	KeyPanelStateRestoreRowConfirm = "panel.state.restore_row_confirm"

	KeyPanelStaffTitle   = "panel.staff.title"
	KeyPanelStaffEyebrow = "panel.staff.eyebrow"
	KeyPanelStaffHeading = "panel.staff.heading"
	KeyPanelStaffLead    = "panel.staff.lead"
	KeyPanelStaffPlayers = "panel.staff.players"
	KeyPanelStaffEmpty   = "panel.staff.empty"
	KeyPanelStaffPlayer  = "panel.staff.player"
	KeyPanelStaffChange  = "panel.staff.change"
	KeyPanelStaffBadJSON = "panel.staff.bad_json"
	KeyPanelStaffBadRank = "panel.staff.bad_rank"

	KeyPanelLogsTitle   = "panel.logs.title"
	KeyPanelLogsEyebrow = "panel.logs.eyebrow"
	KeyPanelLogsHeading = "panel.logs.heading"
	KeyPanelLogsLead    = "panel.logs.lead"
	KeyPanelLogsTail    = "panel.logs.tail"
	KeyPanelLogsShow    = "panel.logs.show"
	KeyPanelLogsPause   = "panel.logs.pause"
	KeyPanelLogsResume  = "panel.logs.resume"
)

const (
	KeyPanelNoticeServerStarted  = "panel.notice.server_started"
	KeyPanelNoticeServerStopped  = "panel.notice.server_stopped"
	KeyPanelNoticeServerRestart  = "panel.notice.server_restarted"
	KeyPanelNoticeBackupCreated  = "panel.notice.backup_created"
	KeyPanelNoticeModsUploaded   = "panel.notice.mods_uploaded"
	KeyPanelNoticeModsDeployed   = "panel.notice.mods_deployed"
	KeyPanelNoticeModsRolledBack = "panel.notice.mods_rolled_back"
	KeyPanelNoticeStateDeployed  = "panel.notice.state_deployed"
	KeyPanelNoticeStateRestored  = "panel.notice.state_restored"
	KeyPanelNoticeStaffSaved     = "panel.notice.staff_saved"

	KeyPanelErrorDoctorBlocked        = "panel.error.doctor_blocked"
	KeyPanelErrorDoctorBlockedDetails = "panel.error.doctor_blocked_details"
	KeyPanelErrorOpsUnavailable       = "panel.error.ops_unavailable"
	KeyPanelErrorInternal             = "panel.error.internal"
	KeyPanelErrorInternalShort        = "panel.error.internal_short"
	KeyPanelErrorInvalidArgument      = "panel.error.invalid_argument"
	KeyPanelErrorNotFound             = "panel.error.not_found"
	KeyPanelErrorLocked               = "panel.error.locked"
	KeyPanelErrorValidation           = "panel.error.validation"
	KeyPanelErrorNoSpace              = "panel.error.no_space"
	KeyPanelErrorServiceUnavailable   = "panel.error.service_unavailable"
	KeyPanelErrorBadForm              = "panel.error.bad_form"

	KeyPanelAuthRateLimited       = "panel.auth.rate_limited"
	KeyPanelAuthBadForm           = "panel.auth.bad_form"
	KeyPanelAuthStaleForm         = "panel.auth.stale_form"
	KeyPanelAuthPasswordRequired  = "panel.auth.password_required"
	KeyPanelAuthPasswordInvalid   = "panel.auth.password_invalid"
	KeyPanelAuthRateLimitedChange = "panel.auth.rate_limited_change"
	KeyPanelAuthBadFormChange     = "panel.auth.bad_form_change"
	KeyPanelAuthPasswordMismatch  = "panel.auth.password_mismatch"
	KeyPanelAuthPasswordSame      = "panel.auth.password_same"
	KeyPanelAuthPasswordShort     = "panel.auth.password_short"
	KeyPanelAuthPasswordLong      = "panel.auth.password_long"
	KeyPanelAuthPasswordWrong     = "panel.auth.password_wrong"
	KeyPanelAuthPasswordSaveFail  = "panel.auth.password_save_failed"

	KeyPanelPublicRateLimited = "panel.public.rate_limited"

	KeyPanelPublicBadRequest      = "panel.public.bad_request"
	KeyPanelPublicNotFound        = "panel.public.not_found"
	KeyPanelPublicCorrupted       = "panel.public.corrupted"
	KeyPanelPublicNoSpace         = "panel.public.no_space"
	KeyPanelPublicUnavailable     = "panel.public.unavailable"
	KeyPanelPublicUnavailableLong = "panel.public.unavailable_long"

	KeyPanelAdminWindowInvalid     = "panel.admin.window_invalid"
	KeyPanelAdminOpsUnavailable    = "panel.admin.ops_unavailable"
	KeyPanelAdminModsMissing       = "panel.admin.mods_missing"
	KeyPanelAdminMetricsStopped    = "panel.admin.metrics_stopped"
	KeyPanelAdminMetricsWindow     = "panel.admin.metrics_window"
	KeyPanelAdminEventsUnavailable = "panel.admin.events_unavailable"
	KeyPanelAdminTailInvalid       = "panel.admin.tail_invalid"
	KeyPanelAdminArchiveInvalid    = "panel.admin.archive_invalid"
	KeyPanelAdminNoPrevious        = "panel.admin.no_previous_archive"
	KeyPanelAdminPreviousInvalid   = "panel.admin.previous_invalid"
	KeyPanelAdminFileMissing       = "panel.admin.file_missing"
	KeyPanelAdminMultipartExpected = "panel.admin.multipart_expected"
	KeyPanelAdminModsTooLarge      = "panel.admin.mods_too_large"
	KeyPanelAdminStateTooLarge     = "panel.admin.state_too_large"
	KeyPanelAdminUploadFailed      = "panel.admin.upload_failed"
	KeyPanelAdminLimitInvalid      = "panel.admin.limit_invalid"
	KeyPanelAdminBeforeInvalid     = "panel.admin.before_invalid"
	KeyPanelAdminBackupUnavailable = "panel.admin.backup_unavailable"
	KeyPanelAdminRestoreUnavail    = "panel.admin.restore_unavailable"
	KeyPanelAdminModesUnsupported  = "panel.admin.mods_import_unsupported"
	KeyPanelAdminStateNameInvalid  = "panel.admin.state_name_invalid"
	KeyPanelAdminSavedFields       = "panel.admin.saved_fields"
	KeyPanelAdminPlayersUnreadable = "panel.admin.players_unreadable"
	KeyPanelAdminPlayerInvalid     = "panel.admin.player_invalid"
	KeyPanelAdminRankInvalid       = "panel.admin.rank_invalid"
	KeyPanelAdminFieldInvalid      = "panel.admin.field_invalid"
	KeyPanelAdminModsNameInvalid   = "panel.admin.mods_name_invalid"
	KeyPanelAdminModsImportUnavail = "panel.admin.mods_import_unavailable"

	KeyPanelStatusRunning     = "panel.status.running"
	KeyPanelStatusStopped     = "panel.status.stopped"
	KeyPanelStatusStarting    = "panel.status.starting"
	KeyPanelStatusUnhealthy   = "panel.status.unhealthy"
	KeyPanelStatusUnavailable = "panel.status.unavailable"
	KeyPanelStatusUnknown     = "panel.status.unknown"

	KeyPanelHealthOK       = "panel.health.ok"
	KeyPanelHealthProblems = "panel.health.problems"
	KeyPanelHealthUnknown  = "panel.health.unknown"

	KeyPanelGroupToday     = "panel.group.today"
	KeyPanelGroupYesterday = "panel.group.yesterday"
	KeyPanelGroupOther     = "panel.group.other"
	KeyPanelGroupWeek      = "panel.group.week"

	KeyPanelMonthJanuary   = "panel.month.january"
	KeyPanelMonthFebruary  = "panel.month.february"
	KeyPanelMonthMarch     = "panel.month.march"
	KeyPanelMonthApril     = "panel.month.april"
	KeyPanelMonthMay       = "panel.month.may"
	KeyPanelMonthJune      = "panel.month.june"
	KeyPanelMonthJuly      = "panel.month.july"
	KeyPanelMonthAugust    = "panel.month.august"
	KeyPanelMonthSeptember = "panel.month.september"
	KeyPanelMonthOctober   = "panel.month.october"
	KeyPanelMonthNovember  = "panel.month.november"
	KeyPanelMonthDecember  = "panel.month.december"

	KeyPanelTimeJustNow   = "panel.time.just_now"
	KeyPanelTimeMinuteAgo = "panel.time.minute_ago"
	KeyPanelTimeHourAgo   = "panel.time.hour_ago"
	KeyPanelTimeDayAgo    = "panel.time.day_ago"
	KeyPanelTimeMonthAgo  = "panel.time.month_ago"
	KeyPanelTimeYearAgo   = "panel.time.year_ago"
)

// Keys for the streaming restore page. These messages explain why an uploaded
// archive was rejected, so they must reach the operator in its own language.
const (
	KeyPanelRestoreUnsupported      = "panel.restore.unsupported"
	KeyPanelRestoreReadFailed       = "panel.restore.read_failed"
	KeyPanelRestoreEmpty            = "panel.restore.empty"
	KeyPanelRestoreNameFailed       = "panel.restore.name_failed"
	KeyPanelRestoreZipUnreadable    = "panel.restore.zip_unreadable"
	KeyPanelRestoreZipTwoFiles      = "panel.restore.zip_two_files"
	KeyPanelRestoreZipRootOnly      = "panel.restore.zip_root_only"
	KeyPanelRestoreZipTwoStates     = "panel.restore.zip_two_states"
	KeyPanelRestoreZipTwoMods       = "panel.restore.zip_two_mods"
	KeyPanelRestoreZipUnexpected    = "panel.restore.zip_unexpected"
	KeyPanelRestoreZipStateAndMods  = "panel.restore.zip_state_and_mods"
	KeyPanelRestoreTempDir          = "panel.restore.temp_dir"
	KeyPanelRestoreUnzipFailed      = "panel.restore.unzip_failed"
	KeyPanelRestoreZipMissing       = "panel.restore.zip_missing"
	KeyPanelRestorePermissions      = "panel.restore.permissions"
	KeyPanelRestoreStateCheck       = "panel.restore.state_check"
	KeyPanelRestoreStateRead        = "panel.restore.state_read"
	KeyPanelRestoreModsCheck        = "panel.restore.mods_check"
	KeyPanelRestoreModsRead         = "panel.restore.mods_read"
	KeyPanelRestoreModsMismatch     = "panel.restore.mods_mismatch"
	KeyPanelRestoreDeployUnexpected = "panel.restore.deploy_unexpected"
	KeyPanelRestoreModsMismatchLive = "panel.restore.mods_mismatch_live"
	KeyPanelRestoreImportMods       = "panel.restore.import_mods"
	KeyPanelRestoreImportState      = "panel.restore.import_state"
	KeyPanelRestoreFileCheck        = "panel.restore.file_check"
	KeyPanelRestoreNotRegular       = "panel.restore.not_regular"
	KeyPanelRestoreFileRead         = "panel.restore.file_read"
	KeyPanelRestoreExistsDiffers    = "panel.restore.exists_differs"
)

// panelCatalogs holds the panel strings. The panel keeps English text in its
// templates and Go code as the source language; init merges these entries into
// catalogs so the launcher and the panel share one lookup path and one
// English fallback rule.
var panelCatalogs = map[Language]map[string]string{
	English: {
		KeyPanelNavAdminSections: "Panel sections",
		KeyPanelNavDashboard:     "Overview",
		KeyPanelNavLogs:          "Logs",
		KeyPanelNavMods:          "Mods",
		KeyPanelNavState:         "Snapshots",
		KeyPanelNavConfig:        "Settings",
		KeyPanelNavStaff:         "Staff",
		KeyPanelNavPublicArchive: "Public archives",
		KeyPanelNavAccount:       "Account",
		KeyPanelNavPassword:      "Change password",
		KeyPanelNavLogout:        "Sign out",
		KeyPanelNavPublicPages:   "Public pages",
		KeyPanelNavHome:          "Home",
		KeyPanelNavArchives:      "Archives",
		KeyPanelNavLogin:         "Panel sign-in",
		KeyPanelNavLanguage:      "Language",

		KeyPanelNoticeRestartRequired: "Restart the server to apply the changes.",

		KeyPanelLoginTitle:    "Sign in — tes3mp-easy",
		KeyPanelLoginEyebrow:  "Administration",
		KeyPanelLoginHeading:  "Panel sign-in",
		KeyPanelLoginHint:     "Enter the server administrator password.",
		KeyPanelLoginPassword: "Password",
		KeyPanelLoginSubmit:   "Sign in",
		KeyPanelLoginBack:     "← Back to the public page",

		KeyPanelPasswordTitle:      "Change password — tes3mp-easy",
		KeyPanelPasswordEyebrow:    "Security",
		KeyPanelPasswordHeading:    "Change password",
		KeyPanelPasswordMustChange: "You signed in with a temporary password. Set a new one to continue.",
		KeyPanelPasswordCurrent:    "Current password",
		KeyPanelPasswordNew:        "New password (at least 12 characters)",
		KeyPanelPasswordConfirm:    "Repeat the new password",
		KeyPanelPasswordSubmit:     "Save password",
		KeyPanelPasswordBack:       "← Back to the panel",

		KeyPanelHomeEyebrow:        "Morrowind server",
		KeyPanelHomeConnect:        "Connect:",
		KeyPanelHomePort:           "port",
		KeyPanelHomeStatusCaption:  "Server status",
		KeyPanelHomeArchivesCTA:    "Archives and snapshots",
		KeyPanelHomeConnection:     "Connection",
		KeyPanelHomeAddress:        "Address",
		KeyPanelHomePortLabel:      "Port",
		KeyPanelHomeStatus:         "Status",
		KeyPanelHomePublicArchives: "Public archives",
		KeyPanelHomePublicHint:     "World snapshots and mod archives are available without signing in.",
		KeyPanelHomeSnapshots:      "Snapshots",
		KeyPanelHomeMods:           "Mods",

		KeyPanelBackupsTitle:           "Archives — tes3mp-easy",
		KeyPanelBackupsEyebrow:         "Public archive",
		KeyPanelBackupsHeading:         "Server archives",
		KeyPanelBackupsLead:            "World snapshots and mod archives are available without signing in. Every snapshot lists the mod pack it was created with.",
		KeyPanelBackupsStateHeading:    "Snapshots",
		KeyPanelBackupsStateHint:       "World, players and config.lua",
		KeyPanelBackupsModsHeading:     "Mods",
		KeyPanelBackupsModsHint:        "Immutable archives",
		KeyPanelBackupsStateEmpty:      "No snapshots yet.",
		KeyPanelBackupsModsEmpty:       "No mod archives yet.",
		KeyPanelBackupsCurrent:         "current",
		KeyPanelBackupsModsLabel:       "Mods:",
		KeyPanelBackupsModsFileMissing: "file missing",
		KeyPanelBackupsLinkUnreadable:  "link unreadable",
		KeyPanelBackupsDownloadState:   "Download snapshot",
		KeyPanelBackupsDownloadBundle:  "Download snapshot and mods (.zip)",
		KeyPanelBackupsDownloadMods:    "Download mods",
		KeyPanelBackupsOlderState:      "Older snapshots",
		KeyPanelBackupsOlderMods:       "Older mods",
		KeyPanelBackupsLimit:           "Entries per column",
		KeyPanelBackupsShow:            "Show",

		KeyPanelCommonArchive:       "Archive",
		KeyPanelCommonSize:          "Size",
		KeyPanelCommonDateUTC:       "Date (UTC)",
		KeyPanelCommonActions:       "Actions",
		KeyPanelCommonAction:        "Action",
		KeyPanelCommonShow:          "Show",
		KeyPanelCommonCurrent:       "current",
		KeyPanelCommonPause:         "Pause",
		KeyPanelCommonResume:        "Resume",
		KeyPanelCommonChangedPrefix: "Changed: ",
		KeyPanelCommonNoChanges:     "No changes",

		KeyPanelDashboardTitle:          "Panel — tes3mp-easy",
		KeyPanelDashboardEyebrow:        "Server",
		KeyPanelDashboardLead:           "The panel is running. Status, resources and the latest events. Charts keep 24 hours in panel memory and never touch the disk.",
		KeyPanelDashboardServer:         "Server",
		KeyPanelDashboardSince:          "since",
		KeyPanelDashboardAddress:        "Address",
		KeyPanelDashboardHealth:         "Health",
		KeyPanelDashboardMods:           "Mods",
		KeyPanelDashboardStart:          "Start",
		KeyPanelDashboardRestart:        "Restart",
		KeyPanelDashboardStop:           "Stop",
		KeyPanelDashboardLogs:           "Logs",
		KeyPanelDashboardConfirmRestart: "Restart the server? Players will be disconnected.",
		KeyPanelDashboardConfirmStop:    "Stop the server? Players will be disconnected.",
		KeyPanelDashboardDoctorHint:     "Mutations are blocked while <code>doctor</code> reports <code>error</code>. Metrics never affect this.",
		KeyPanelDashboardResources:      "Resources",
		KeyPanelDashboardWindowAria:     "Chart window",
		KeyPanelDashboardWindow1h:       "1 h",
		KeyPanelDashboardWindow6h:       "6 h",
		KeyPanelDashboardWindow24h:      "24 h",
		KeyPanelDashboardRAM:            "Game RAM",
		KeyPanelDashboardGameCPU:        "Game CPU",
		KeyPanelDashboardSysCPU:         "System CPU",
		KeyPanelDashboardSysRAM:         "System RAM",
		KeyPanelDashboardSwap:           "swap",
		KeyPanelDashboardDiskFree:       "free on disk",
		KeyPanelDashboardNoCgroup:       "The game cgroup is unreadable: the server is stopped. System metrics are available.",
		KeyPanelDashboardRAMChart:       "Game RAM",
		KeyPanelDashboardRAMHint:        "%s; peaks are visible, gaps are missing samples.",
		KeyPanelDashboardCPUChart:       "Game and system CPU",
		KeyPanelDashboardCPUHint:        "Game — <span class=\"tag\">amber</span>, system — <span class=\"tag\">blue</span>.",
		KeyPanelDashboardDiskChart:      "Free disk space",
		KeyPanelDashboardDiskHint:       "%s; free space in the selected window.",
		KeyPanelDashboardNoScript:       "Charts refresh every 15 seconds through <code>admin.js</code>; without JavaScript the values from the page load are shown.",
		KeyPanelDashboardEvents:         "Latest events",
		KeyPanelDashboardEventsEmpty:    "No events yet.",
		KeyPanelDashboardOpenLogs:       "Open logs",
		KeyPanelDashboardAuditHint:      "Audit without passwords or file contents",
		KeyPanelDashboardQuickActions:   "Quick actions",
		KeyPanelDashboardCreateState:    "Create snapshot",
		KeyPanelDashboardUploadMods:     "Upload mods",
		KeyPanelDashboardRestore:        "Restore",

		KeyPanelStateTitle:          "Snapshots — tes3mp-easy",
		KeyPanelStateEyebrow:        "Backups",
		KeyPanelStateHeading:        "Snapshots",
		KeyPanelStateLead:           "A snapshot keeps players, the world and config.lua. The server password and banlist.json stay in the live configs and never enter a snapshot. Mods are separate immutable archives referenced by the snapshot.",
		KeyPanelStateMissingMods:    "The current mod archive is missing. Restoring is unavailable until mods are deployed.",
		KeyPanelStateCurrentMods:    "Current mods: <code>%s</code>",
		KeyPanelStateRestoreHeading: "Restore from a downloaded archive",
		KeyPanelStateRestoreHint:    "A ZIP with a snapshot and its mods imports the mods and restores the snapshot; a standalone state <code>.tar.gz</code> works only on the active mod pack.",
		KeyPanelStateRestoreConfirm: "Restore the snapshot from the archive? players/, world/ and config.lua will be replaced.",
		KeyPanelStateFileLabel:      "Archive .zip or state .tar.gz, up to 1 GiB",
		KeyPanelStateUploadRestore:  "Upload and restore",
		KeyPanelStateWhatChanges:    "What exactly is replaced",
		KeyPanelStateWhatChangesTxt: "players/, world/ and config.lua are replaced; the password in tes3mp-server-default.cfg and banlist.json stay untouched. Mods are replaced only from a ZIP. A running server is restarted. Before restoring, ops creates state-predeploy-*.",
		KeyPanelStateCreate:         "Create snapshot",
		KeyPanelStateManageMods:     "Manage mods",
		KeyPanelStateLimit:          "Rows per page",
		KeyPanelStateEmpty:          "No snapshots yet.",
		KeyPanelStateMods:           "Mods",
		KeyPanelStateModsMissing:    "Mods missing",
		KeyPanelStateModsActive:     "active",
		KeyPanelStateModsInactive:   "not active",
		KeyPanelStateDownloadState:  "Download snapshot",
		KeyPanelStateDownloadMods:   "Download mods",
		KeyPanelStateDeployMods:     "Deploy mods",
		KeyPanelStateDeployConfirm:  "Deploy mods %s? The server will be restarted.",
		KeyPanelStateRestoreButton:  "Restore",
		KeyPanelStateDeployFirst:    "Deploy the referenced mod archive first",
		KeyPanelStateOlder:          "Show older",

		KeyPanelModsTitle:           "Mods — tes3mp-easy",
		KeyPanelModsEyebrow:         "Content",
		KeyPanelModsHeading:         "Mods",
		KeyPanelModsLead:            "Upload a mod archive, review it and deploy it to the server.",
		KeyPanelModsCurrentHeading:  "Current archive",
		KeyPanelModsNotDeployed:     "No mods deployed yet.",
		KeyPanelModsUploadHeading:   "Upload archive",
		KeyPanelModsUploadHint:      "Archive up to 512 MiB. After the import it still has to be deployed separately.",
		KeyPanelModsFormat:          "Archive layout",
		KeyPanelModsFormatRoot:      "The root holds only <code>plugins/</code> and <code>scripts/</code>; <code>requiredDataFiles.json</code> lives inside <code>plugins/</code>, never at the root.",
		KeyPanelModsFormatPlugins:   "Plugins: <code>.esp</code>, <code>.esm</code>, <code>.omwaddon</code>, <code>.omwscripts</code>, <code>.omwgame</code>. Every mod plugin listed in the JSON must be in <code>plugins/</code> and vice versa (except the three base ESMs).",
		KeyPanelModsFormatCRC:       "CRC32 in the JSON is <code>0x1A2B3C4D</code>: eight upper-case hex digits; an empty <code>[]</code> only checks that the file exists.",
		KeyPanelModsFormatBaseESM:   "Never ship <code>Morrowind.esm</code>, <code>Tribunal.esm</code> or <code>Bloodmoon.esm</code> in the archive.",
		KeyPanelModsFormatScripts:   "<code>scripts/</code> holds top-level <code>.lua</code> files only; names use letters, digits, <code>_</code> and <code>-</code>.",
		KeyPanelModsExample:         "Example <code>plugins/requiredDataFiles.json</code>:",
		KeyPanelModsFileLabel:       "Archive .tar.gz",
		KeyPanelModsUpload:          "Upload",
		KeyPanelModsArchivesHeading: "Mod archives",
		KeyPanelModsEmpty:           "No mod archives yet.",
		KeyPanelModsCurrentTag:      "(current)",
		KeyPanelModsDeploy:          "Deploy",
		KeyPanelModsRollbackHeading: "Rollback",
		KeyPanelModsRollbackText:    "The rollback deploys the previous archive <code>%s</code>. A running server is restarted.",
		KeyPanelModsRollback:        "Rollback",

		KeyPanelConfigTitle:      "Settings — tes3mp-easy",
		KeyPanelConfigEyebrow:    "Configuration",
		KeyPanelConfigHeading:    "Settings",
		KeyPanelConfigLead:       "The form is submitted as a whole. The panel compares the fields and sends only the changes.",
		KeyPanelConfigHint:       "With nothing changed the button stays disabled and shows “No changes”; no extra mutation is performed.",
		KeyPanelConfigFieldError: "Check this field: %s",
		KeyPanelConfigYes:        "Yes",
		KeyPanelConfigNo:         "No",
		KeyPanelConfigSave:       "Save changes",
		KeyPanelConfigReset:      "Cancel",
		KeyPanelConfigStatusIdle: "No changes",

		KeyPanelConfigSectionServer:  "Server",
		KeyPanelConfigSectionGame:    "Game settings",
		KeyPanelConfigSectionBanlist: "Banlist",
		KeyPanelConfigGameNote:       "Tables and lists are shown read-only.",
		KeyPanelConfigHostname:       "Server name",
		KeyPanelConfigPort:           "Port (1024..65535)",
		KeyPanelConfigMaxPlayers:     "Max players (1..64)",
		KeyPanelConfigPassword:       "Server password (empty = no password)",
		KeyPanelConfigLogLevel:       "Log level (0..4)",
		KeyPanelConfigBanNames:       "Player names (one per line)",
		KeyPanelConfigBanIPs:         "IP addresses (one per line)",

		KeyPanelStateRestoreRowConfirm: "Restore snapshot %s? The current players/, world/ and config.lua will be replaced.",

		KeyPanelStaffTitle:   "Staff — tes3mp-easy",
		KeyPanelStaffEyebrow: "Players",
		KeyPanelStaffHeading: "Staff",
		KeyPanelStaffLead:    "Assign ranks to players. The change applies after the server restart.",
		KeyPanelStaffPlayers: "Players",
		KeyPanelStaffEmpty:   "No players yet.",
		KeyPanelStaffPlayer:  "Player",
		KeyPanelStaffChange:  "Change",
		KeyPanelStaffBadJSON: "broken JSON",
		KeyPanelStaffBadRank: "invalid rank",

		KeyPanelLogsTitle:   "Logs — tes3mp-easy",
		KeyPanelLogsEyebrow: "Diagnostics",
		KeyPanelLogsHeading: "Logs",
		KeyPanelLogsLead:    "The latest panel and game server log lines refresh automatically.",
		KeyPanelLogsTail:    "Last lines",
		KeyPanelLogsShow:    "Show",
		KeyPanelLogsPause:   "Pause",
		KeyPanelLogsResume:  "Resume",

		KeyPanelNoticeServerStarted:  "The server is starting.",
		KeyPanelNoticeServerStopped:  "The server is stopped.",
		KeyPanelNoticeServerRestart:  "The server is restarting.",
		KeyPanelNoticeBackupCreated:  "Backup created.",
		KeyPanelNoticeModsUploaded:   "The mod archive is uploaded and imported.",
		KeyPanelNoticeModsDeployed:   "Mods deployed.",
		KeyPanelNoticeModsRolledBack: "The previous mod archive is deployed.",
		KeyPanelNoticeStateDeployed:  "Snapshot restored.",
		KeyPanelNoticeStateRestored:  "Snapshot and mods restored from the uploaded archive.",
		KeyPanelNoticeStaffSaved:     "Rank changed. Restart the server to apply it.",

		KeyPanelErrorDoctorBlocked:        "doctor reported an error; the mutation is blocked.",
		KeyPanelErrorDoctorBlockedDetails: "doctor reported an error, the mutation is blocked: %s",
		KeyPanelErrorOpsUnavailable:       "ops is not configured: server management is unavailable.",
		KeyPanelErrorInternal:             "Internal error. Details are in the panel log.",
		KeyPanelErrorInternalShort:        "Internal error.",
		KeyPanelErrorInvalidArgument:      "Check the name or the value.",
		KeyPanelErrorNotFound:             "Object not found.",
		KeyPanelErrorLocked:               "Another operation is already running. Try again later.",
		KeyPanelErrorValidation:           "The data failed validation.",
		KeyPanelErrorNoSpace:              "Not enough free space.",
		KeyPanelErrorServiceUnavailable:   "systemd, journald or the unit is unavailable.",
		KeyPanelErrorBadForm:              "Invalid form.",

		KeyPanelAuthRateLimited:       "Too many sign-in attempts. Wait and try again.",
		KeyPanelAuthBadForm:           "Invalid sign-in form.",
		KeyPanelAuthStaleForm:         "The form is stale. Reload the page and try again.",
		KeyPanelAuthPasswordRequired:  "Enter the password.",
		KeyPanelAuthPasswordInvalid:   "Wrong password.",
		KeyPanelAuthRateLimitedChange: "Too many attempts. Wait and try again.",
		KeyPanelAuthBadFormChange:     "Invalid form.",
		KeyPanelAuthPasswordMismatch:  "The passwords do not match.",
		KeyPanelAuthPasswordSame:      "The new password must differ from the current one.",
		KeyPanelAuthPasswordShort:     "The new password must be at least 12 characters long.",
		KeyPanelAuthPasswordLong:      "The new password is too long.",
		KeyPanelAuthPasswordWrong:     "The current password is wrong.",
		KeyPanelAuthPasswordSaveFail:  "The password could not be saved.",

		KeyPanelPublicRateLimited: "Too many requests. Try again later.",

		KeyPanelPublicBadRequest:      "Check the request parameters.",
		KeyPanelPublicNotFound:        "The requested archive was not found.",
		KeyPanelPublicCorrupted:       "The archive or the configuration is corrupted.",
		KeyPanelPublicNoSpace:         "Not enough free space for the operation.",
		KeyPanelPublicUnavailable:     "The server is temporarily unavailable.",
		KeyPanelPublicUnavailableLong: "The server is temporarily unavailable: systemd or the unit is not responding.",

		KeyPanelAdminWindowInvalid:     "The window parameter must be one of: 1h, 6h, 24h.",
		KeyPanelAdminOpsUnavailable:    "ops is not configured: server management is unavailable.",
		KeyPanelAdminModsMissing:       "The current mod archive is missing.",
		KeyPanelAdminMetricsStopped:    "Metrics collection is not running.",
		KeyPanelAdminMetricsWindow:     "Unknown metrics window.",
		KeyPanelAdminEventsUnavailable: "The latest events are unavailable.",
		KeyPanelAdminTailInvalid:       "The tail parameter must be a number from 1 to 2000.",
		KeyPanelAdminArchiveInvalid:    "Invalid archive name.",
		KeyPanelAdminNoPrevious:        "There is no previous archive to roll back to.",
		KeyPanelAdminPreviousInvalid:   "The previous archive has an invalid name.",
		KeyPanelAdminFileMissing:       "The form has no archive file.",
		KeyPanelAdminMultipartExpected: "Expected a multipart form with an archive.",
		KeyPanelAdminModsTooLarge:      "The archive is larger than 512 MiB.",
		KeyPanelAdminStateTooLarge:     "The archive is larger than 1 GiB.",
		KeyPanelAdminUploadFailed:      "The uploaded file could not be saved.",
		KeyPanelAdminLimitInvalid:      "The limit parameter must be a number from 1 to 500.",
		KeyPanelAdminBeforeInvalid:     "Invalid archive name in the before parameter.",
		KeyPanelAdminBackupUnavailable: "ops is not configured: creating a backup is unavailable.",
		KeyPanelAdminRestoreUnavail:    "ops is not configured: restoring is unavailable.",
		KeyPanelAdminModesUnsupported:  "ops cannot import snapshots and mods.",
		KeyPanelAdminStateNameInvalid:  "Invalid state archive name.",
		KeyPanelAdminSavedFields:       "Fields saved: %d.",
		KeyPanelAdminPlayersUnreadable: "The player list could not be read.",
		KeyPanelAdminPlayerInvalid:     "Invalid player name.",
		KeyPanelAdminRankInvalid:       "The rank must be a number from 0 to 3.",
		KeyPanelAdminFieldInvalid:      "Check the value of the field %s.",
		KeyPanelAdminModsNameInvalid:   "Invalid mod archive name.",
		KeyPanelAdminModsImportUnavail: "ops is not configured: importing mods is unavailable.",

		KeyPanelStatusRunning:     "running",
		KeyPanelStatusStopped:     "stopped",
		KeyPanelStatusStarting:    "starting",
		KeyPanelStatusUnhealthy:   "unhealthy",
		KeyPanelStatusUnavailable: "unavailable",
		KeyPanelStatusUnknown:     "unknown",

		KeyPanelHealthOK:       "ok",
		KeyPanelHealthProblems: "problems",
		KeyPanelHealthUnknown:  "unknown",

		KeyPanelGroupToday:     "Today",
		KeyPanelGroupYesterday: "Yesterday",
		KeyPanelGroupOther:     "Other",
		KeyPanelGroupWeek:      "Week %d, %d",

		KeyPanelMonthJanuary:   "January",
		KeyPanelMonthFebruary:  "February",
		KeyPanelMonthMarch:     "March",
		KeyPanelMonthApril:     "April",
		KeyPanelMonthMay:       "May",
		KeyPanelMonthJune:      "June",
		KeyPanelMonthJuly:      "July",
		KeyPanelMonthAugust:    "August",
		KeyPanelMonthSeptember: "September",
		KeyPanelMonthOctober:   "October",
		KeyPanelMonthNovember:  "November",
		KeyPanelMonthDecember:  "December",

		KeyPanelTimeJustNow:              "just now",
		KeyPanelTimeMinuteAgo + ".one":   "%d minute ago",
		KeyPanelTimeMinuteAgo + ".other": "%d minutes ago",
		KeyPanelTimeHourAgo + ".one":     "%d hour ago",
		KeyPanelTimeHourAgo + ".other":   "%d hours ago",
		KeyPanelTimeDayAgo + ".one":      "%d day ago",
		KeyPanelTimeDayAgo + ".other":    "%d days ago",
		KeyPanelTimeMonthAgo + ".one":    "%d month ago",
		KeyPanelTimeMonthAgo + ".other":  "%d months ago",
		KeyPanelTimeYearAgo + ".one":     "%d year ago",
		KeyPanelTimeYearAgo + ".other":   "%d years ago",
		KeyPanelTimeYearAgo + ".few":     "%d years ago",
		KeyPanelTimeYearAgo + ".many":    "%d years ago",
		KeyPanelTimeMinuteAgo + ".few":   "%d minutes ago",
		KeyPanelTimeMinuteAgo + ".many":  "%d minutes ago",
		KeyPanelTimeHourAgo + ".few":     "%d hours ago",
		KeyPanelTimeHourAgo + ".many":    "%d hours ago",
		KeyPanelTimeDayAgo + ".few":      "%d days ago",
		KeyPanelTimeDayAgo + ".many":     "%d days ago",
		KeyPanelTimeMonthAgo + ".few":    "%d months ago",
		KeyPanelTimeMonthAgo + ".many":   "%d months ago",

		KeyPanelRestoreUnsupported:      "Only a ZIP with a snapshot and its mods or a state .tar.gz is supported.",
		KeyPanelRestoreReadFailed:       "The uploaded file could not be read.",
		KeyPanelRestoreEmpty:            "The uploaded file is empty or corrupted.",
		KeyPanelRestoreNameFailed:       "Could not pick a name for the imported snapshot.",
		KeyPanelRestoreZipUnreadable:    "The ZIP cannot be read: %v",
		KeyPanelRestoreZipTwoFiles:      "The ZIP must contain exactly two files: a snapshot and its mods.",
		KeyPanelRestoreZipRootOnly:      "The ZIP must contain only two archives at its root.",
		KeyPanelRestoreZipTwoStates:     "The ZIP contains more than one state archive.",
		KeyPanelRestoreZipTwoMods:       "The ZIP contains more than one mod archive.",
		KeyPanelRestoreZipUnexpected:    "Unexpected file in the ZIP: %s",
		KeyPanelRestoreZipStateAndMods:  "The ZIP must contain a state archive and a mod archive.",
		KeyPanelRestoreTempDir:          "The temporary directory could not be prepared.",
		KeyPanelRestoreUnzipFailed:      "The ZIP was not unpacked: %v",
		KeyPanelRestoreZipMissing:       "The ZIP does not contain the expected file %s.",
		KeyPanelRestorePermissions:      "The permissions of the unpacked archive could not be set.",
		KeyPanelRestoreStateCheck:       "The state archive failed validation: %v",
		KeyPanelRestoreStateRead:        "The state archive could not be read.",
		KeyPanelRestoreModsCheck:        "The mod archive failed validation: %v",
		KeyPanelRestoreModsRead:         "The mod archive could not be read.",
		KeyPanelRestoreModsMismatch:     "The mod archive does not match the snapshot: the state expects %s (%s), the ZIP holds %s (%s).",
		KeyPanelRestoreDeployUnexpected: "ops deployed an unexpected mod archive %q.",
		KeyPanelRestoreModsMismatchLive: "The live mod pack does not match the snapshot: %v",
		KeyPanelRestoreImportMods:       "ops imported an unexpected mod archive %q.",
		KeyPanelRestoreImportState:      "ops imported an unexpected snapshot %q.",
		KeyPanelRestoreFileCheck:        "Could not check %s.",
		KeyPanelRestoreNotRegular:       "%s already exists and is not a regular file.",
		KeyPanelRestoreFileRead:         "Could not read %s.",
		KeyPanelRestoreExistsDiffers:    "%s already exists with different content; rename the archive in the ZIP or delete the old file manually.",
	},

	Russian: {
		KeyPanelNavAdminSections: "Разделы панели",
		KeyPanelNavDashboard:     "Обзор",
		KeyPanelNavLogs:          "Логи",
		KeyPanelNavMods:          "Моды",
		KeyPanelNavState:         "Состояния",
		KeyPanelNavConfig:        "Настройки",
		KeyPanelNavStaff:         "Staff",
		KeyPanelNavPublicArchive: "Публичные архивы",
		KeyPanelNavAccount:       "Аккаунт",
		KeyPanelNavPassword:      "Сменить пароль",
		KeyPanelNavLogout:        "Выйти",
		KeyPanelNavPublicPages:   "Публичные страницы",
		KeyPanelNavHome:          "Главная",
		KeyPanelNavArchives:      "Архивы",
		KeyPanelNavLogin:         "Вход в панель",
		KeyPanelNavLanguage:      "Язык",

		KeyPanelNoticeRestartRequired: "Нужен перезапуск сервера, чтобы применить изменения.",

		KeyPanelLoginTitle:    "Вход — tes3mp-easy",
		KeyPanelLoginEyebrow:  "Администрирование",
		KeyPanelLoginHeading:  "Вход в панель",
		KeyPanelLoginHint:     "Введите пароль администратора сервера.",
		KeyPanelLoginPassword: "Пароль",
		KeyPanelLoginSubmit:   "Войти",
		KeyPanelLoginBack:     "← На главную",

		KeyPanelPasswordTitle:      "Смена пароля — tes3mp-easy",
		KeyPanelPasswordEyebrow:    "Безопасность",
		KeyPanelPasswordHeading:    "Смена пароля",
		KeyPanelPasswordMustChange: "Вы вошли с временным паролем. Задайте новый пароль, чтобы продолжить.",
		KeyPanelPasswordCurrent:    "Текущий пароль",
		KeyPanelPasswordNew:        "Новый пароль (не менее 12 символов)",
		KeyPanelPasswordConfirm:    "Повторите новый пароль",
		KeyPanelPasswordSubmit:     "Сохранить пароль",
		KeyPanelPasswordBack:       "← Вернуться в панель",

		KeyPanelHomeEyebrow:        "Сервер Morrowind",
		KeyPanelHomeConnect:        "Подключение:",
		KeyPanelHomePort:           "порт",
		KeyPanelHomeStatusCaption:  "Статус сервера",
		KeyPanelHomeArchivesCTA:    "Архивы и состояния",
		KeyPanelHomeConnection:     "Подключение",
		KeyPanelHomeAddress:        "Адрес",
		KeyPanelHomePortLabel:      "Порт",
		KeyPanelHomeStatus:         "Статус",
		KeyPanelHomePublicArchives: "Публичные архивы",
		KeyPanelHomePublicHint:     "Состояния мира и архивы модов доступны без входа.",
		KeyPanelHomeSnapshots:      "Состояния",
		KeyPanelHomeMods:           "Моды",

		KeyPanelBackupsTitle:           "Архивы — tes3mp-easy",
		KeyPanelBackupsEyebrow:         "Публичный архив",
		KeyPanelBackupsHeading:         "Архивы сервера",
		KeyPanelBackupsLead:            "Состояния мира и архивы модов доступны без входа. У каждого состояния указан пакет модов, с которым оно создано.",
		KeyPanelBackupsStateHeading:    "Состояния",
		KeyPanelBackupsStateHint:       "Мир, игроки и config.lua",
		KeyPanelBackupsModsHeading:     "Моды",
		KeyPanelBackupsModsHint:        "Неизменяемые архивы",
		KeyPanelBackupsStateEmpty:      "Состояний пока нет.",
		KeyPanelBackupsModsEmpty:       "Архивов модов пока нет.",
		KeyPanelBackupsCurrent:         "текущий",
		KeyPanelBackupsModsLabel:       "Моды:",
		KeyPanelBackupsModsFileMissing: "файл отсутствует",
		KeyPanelBackupsLinkUnreadable:  "ссылка не читается",
		KeyPanelBackupsDownloadState:   "Скачать состояние",
		KeyPanelBackupsDownloadBundle:  "Скачать состояние и моды (.zip)",
		KeyPanelBackupsDownloadMods:    "Скачать моды",
		KeyPanelBackupsOlderState:      "Более старые состояния",
		KeyPanelBackupsOlderMods:       "Более старые моды",
		KeyPanelBackupsLimit:           "Записей в каждой колонке",
		KeyPanelBackupsShow:            "Показать",

		KeyPanelCommonArchive:       "Архив",
		KeyPanelCommonSize:          "Размер",
		KeyPanelCommonDateUTC:       "Дата (UTC)",
		KeyPanelCommonActions:       "Действия",
		KeyPanelCommonAction:        "Действие",
		KeyPanelCommonShow:          "Показать",
		KeyPanelCommonCurrent:       "текущий",
		KeyPanelCommonPause:         "Пауза",
		KeyPanelCommonResume:        "Продолжить",
		KeyPanelCommonChangedPrefix: "Изменено: ",
		KeyPanelCommonNoChanges:     "Изменений нет",

		KeyPanelDashboardTitle:          "Панель — tes3mp-easy",
		KeyPanelDashboardEyebrow:        "Сервер",
		KeyPanelDashboardLead:           "Панель запущена. Статус, ресурсы и последние события. Графики — 24 часа в памяти панели, на диск не пишутся.",
		KeyPanelDashboardServer:         "Сервер",
		KeyPanelDashboardSince:          "с",
		KeyPanelDashboardAddress:        "Адрес",
		KeyPanelDashboardHealth:         "Health",
		KeyPanelDashboardMods:           "Моды",
		KeyPanelDashboardStart:          "Запустить",
		KeyPanelDashboardRestart:        "Перезапустить",
		KeyPanelDashboardStop:           "Остановить",
		KeyPanelDashboardLogs:           "Логи",
		KeyPanelDashboardConfirmRestart: "Перезапустить сервер? Игроки будут отключены.",
		KeyPanelDashboardConfirmStop:    "Остановить сервер? Игроки будут отключены.",
		KeyPanelDashboardDoctorHint:     "Мутации блокируются, пока <code>doctor</code> в статусе <code>error</code>. Метрики на это не влияют.",
		KeyPanelDashboardResources:      "Ресурсы",
		KeyPanelDashboardWindowAria:     "Окно графиков",
		KeyPanelDashboardWindow1h:       "1 ч",
		KeyPanelDashboardWindow6h:       "6 ч",
		KeyPanelDashboardWindow24h:      "24 ч",
		KeyPanelDashboardRAM:            "RAM игры",
		KeyPanelDashboardGameCPU:        "CPU игры",
		KeyPanelDashboardSysCPU:         "CPU системы",
		KeyPanelDashboardSysRAM:         "RAM системы",
		KeyPanelDashboardSwap:           "swap",
		KeyPanelDashboardDiskFree:       "свободно на диске",
		KeyPanelDashboardNoCgroup:       "cgroup игры не читается: сервер остановлен. Системные метрики доступны.",
		KeyPanelDashboardRAMChart:       "RAM игры",
		KeyPanelDashboardRAMHint:        "%s; пики видны, пропуски — разрыв.",
		KeyPanelDashboardCPUChart:       "CPU игры и системы",
		KeyPanelDashboardCPUHint:        "Игра — <span class=\"tag\">янтарный</span>, система — <span class=\"tag\">синий</span>.",
		KeyPanelDashboardDiskChart:      "Свободно на диске",
		KeyPanelDashboardDiskHint:       "%s; свободное место в выбранном окне.",
		KeyPanelDashboardNoScript:       "Графики обновляются раз в 15 секунд через <code>admin.js</code>; без JavaScript показаны значения на момент загрузки.",
		KeyPanelDashboardEvents:         "Последние события",
		KeyPanelDashboardEventsEmpty:    "Событий пока нет.",
		KeyPanelDashboardOpenLogs:       "Открыть логи",
		KeyPanelDashboardAuditHint:      "Аудит без паролей и содержимого файлов",
		KeyPanelDashboardQuickActions:   "Быстрые действия",
		KeyPanelDashboardCreateState:    "Создать состояние",
		KeyPanelDashboardUploadMods:     "Загрузить моды",
		KeyPanelDashboardRestore:        "Восстановить",

		KeyPanelStateTitle:          "Состояния — tes3mp-easy",
		KeyPanelStateEyebrow:        "Резервные копии",
		KeyPanelStateHeading:        "Состояния",
		KeyPanelStateLead:           "Состояние хранит игроков, мир и config.lua. Пароль сервера и banlist.json остаются в живых конфигах и в state не попадают. Моды — отдельные неизменяемые архивы, на которые ссылается состояние.",
		KeyPanelStateMissingMods:    "Текущий архив модов не найден. Восстановление недоступно, пока моды не задеплоены.",
		KeyPanelStateCurrentMods:    "Текущие моды: <code>%s</code>",
		KeyPanelStateRestoreHeading: "Восстановить из скачанного архива",
		KeyPanelStateRestoreHint:    "ZIP «состояние + моды» импортирует моды и восстановит состояние; отдельный state <code>.tar.gz</code> — только на активном пакете модов.",
		KeyPanelStateRestoreConfirm: "Восстановить состояние из архива? players/, world/ и config.lua будут заменены.",
		KeyPanelStateFileLabel:      "Архив .zip или state .tar.gz, до 1 ГиБ",
		KeyPanelStateUploadRestore:  "Загрузить и восстановить",
		KeyPanelStateWhatChanges:    "Что именно заменяется",
		KeyPanelStateWhatChangesTxt: "players/, world/ и config.lua заменяются; пароль в tes3mp-server-default.cfg и banlist.json не меняются. Моды подменяются только из ZIP. Если сервер работает, он будет перезапущен. Перед восстановлением ops создаст state-predeploy-*.",
		KeyPanelStateCreate:         "Создать состояние",
		KeyPanelStateManageMods:     "Управление модами",
		KeyPanelStateLimit:          "Записей на страницу",
		KeyPanelStateEmpty:          "Состояний пока нет.",
		KeyPanelStateMods:           "Моды",
		KeyPanelStateModsMissing:    "Моды отсутствуют",
		KeyPanelStateModsActive:     "активны",
		KeyPanelStateModsInactive:   "не активны",
		KeyPanelStateDownloadState:  "Скачать состояние",
		KeyPanelStateDownloadMods:   "Скачать моды",
		KeyPanelStateDeployMods:     "Задеплоить моды",
		KeyPanelStateDeployConfirm:  "Задеплоить моды %s? Сервер будет перезапущен.",
		KeyPanelStateRestoreButton:  "Восстановить",
		KeyPanelStateDeployFirst:    "Сначала задеплойте указанный архив модов",
		KeyPanelStateOlder:          "Показать более старые",

		KeyPanelModsTitle:           "Моды — tes3mp-easy",
		KeyPanelModsEyebrow:         "Контент",
		KeyPanelModsHeading:         "Моды",
		KeyPanelModsLead:            "Загрузите архив модов, проверьте его и задеплойте на сервер.",
		KeyPanelModsCurrentHeading:  "Текущий архив",
		KeyPanelModsNotDeployed:     "Моды ещё не задеплоены.",
		KeyPanelModsUploadHeading:   "Загрузить архив",
		KeyPanelModsUploadHint:      "Архив до 512 MiB. После импорта его нужно задеплоить отдельно.",
		KeyPanelModsFormat:          "Формат архива",
		KeyPanelModsFormatRoot:      "В корне только <code>plugins/</code> и <code>scripts/</code>; <code>requiredDataFiles.json</code> лежит внутри <code>plugins/</code>, не в корне.",
		KeyPanelModsFormatPlugins:   "Плагины: <code>.esp</code>, <code>.esm</code>, <code>.omwaddon</code>, <code>.omwscripts</code>, <code>.omwgame</code>. Каждый модовый плагин из JSON должен быть в <code>plugins/</code>, и наоборот (кроме трёх базовых ESM).",
		KeyPanelModsFormatCRC:       "CRC32 в JSON — <code>0x1A2B3C4D</code>: 8 hex-цифр в верхнем регистре; пустой массив <code>[]</code> означает проверку только наличия файла.",
		KeyPanelModsFormatBaseESM:   "<code>Morrowind.esm</code>, <code>Tribunal.esm</code>, <code>Bloodmoon.esm</code> в архив не включать.",
		KeyPanelModsFormatScripts:   "В <code>scripts/</code> только <code>.lua</code> верхнего уровня; имена — буквы, цифры, <code>_</code>, <code>-</code>.",
		KeyPanelModsExample:         "Пример <code>plugins/requiredDataFiles.json</code>:",
		KeyPanelModsFileLabel:       "Архив .tar.gz",
		KeyPanelModsUpload:          "Загрузить",
		KeyPanelModsArchivesHeading: "Архивы модов",
		KeyPanelModsEmpty:           "Архивов модов пока нет.",
		KeyPanelModsCurrentTag:      "(текущий)",
		KeyPanelModsDeploy:          "Deploy",
		KeyPanelModsRollbackHeading: "Откат",
		KeyPanelModsRollbackText:    "Откат задеплоит предыдущий архив <code>%s</code>. Если сервер запущен, он будет перезапущен.",
		KeyPanelModsRollback:        "Rollback",

		KeyPanelConfigTitle:      "Настройки — tes3mp-easy",
		KeyPanelConfigEyebrow:    "Конфигурация",
		KeyPanelConfigHeading:    "Настройки",
		KeyPanelConfigLead:       "Форма отправляется целиком. Панель сравнивает поля и передаёт только изменения.",
		KeyPanelConfigHint:       "Если ничего не менять, кнопка неактивна и показано «Изменений нет»; лишняя мутация не выполняется.",
		KeyPanelConfigFieldError: "Проверьте поле: %s",
		KeyPanelConfigYes:        "Да",
		KeyPanelConfigNo:         "Нет",
		KeyPanelConfigSave:       "Сохранить изменения",
		KeyPanelConfigReset:      "Отменить",
		KeyPanelConfigStatusIdle: "Изменений нет",

		KeyPanelConfigSectionServer:  "Сервер",
		KeyPanelConfigSectionGame:    "Игровые настройки",
		KeyPanelConfigSectionBanlist: "Банлист",
		KeyPanelConfigGameNote:       "Таблицы и списки показываются только для чтения.",
		KeyPanelConfigHostname:       "Имя сервера",
		KeyPanelConfigPort:           "Порт (1024..65535)",
		KeyPanelConfigMaxPlayers:     "Максимум игроков (1..64)",
		KeyPanelConfigPassword:       "Пароль сервера (пусто = без пароля)",
		KeyPanelConfigLogLevel:       "Уровень логов (0..4)",
		KeyPanelConfigBanNames:       "Имена игроков (по одной на строку)",
		KeyPanelConfigBanIPs:         "IP-адреса (по одному на строку)",

		KeyPanelStateRestoreRowConfirm: "Восстановить состояние %s? Текущие players/, world/ и config.lua будут заменены.",

		KeyPanelStaffTitle:   "Staff — tes3mp-easy",
		KeyPanelStaffEyebrow: "Игроки",
		KeyPanelStaffHeading: "Staff",
		KeyPanelStaffLead:    "Назначайте ранги игрокам. Изменение применится после перезапуска сервера.",
		KeyPanelStaffPlayers: "Игроки",
		KeyPanelStaffEmpty:   "Игроков пока нет.",
		KeyPanelStaffPlayer:  "Игрок",
		KeyPanelStaffChange:  "Изменить",
		KeyPanelStaffBadJSON: "повреждённый JSON",
		KeyPanelStaffBadRank: "некорректный ранг",

		KeyPanelLogsTitle:   "Логи — tes3mp-easy",
		KeyPanelLogsEyebrow: "Диагностика",
		KeyPanelLogsHeading: "Логи",
		KeyPanelLogsLead:    "Последние строки журнала panel и игрового сервера обновляются автоматически.",
		KeyPanelLogsTail:    "Последних строк",
		KeyPanelLogsShow:    "Показать",
		KeyPanelLogsPause:   "Пауза",
		KeyPanelLogsResume:  "Продолжить",

		KeyPanelNoticeServerStarted:  "Сервер запускается.",
		KeyPanelNoticeServerStopped:  "Сервер остановлен.",
		KeyPanelNoticeServerRestart:  "Сервер перезапускается.",
		KeyPanelNoticeBackupCreated:  "Резервная копия создана.",
		KeyPanelNoticeModsUploaded:   "Архив модов загружен и импортирован.",
		KeyPanelNoticeModsDeployed:   "Моды задеплоены.",
		KeyPanelNoticeModsRolledBack: "Предыдущий архив модов задеплоен.",
		KeyPanelNoticeStateDeployed:  "Состояние восстановлено.",
		KeyPanelNoticeStateRestored:  "Состояние и моды восстановлены из загруженного архива.",
		KeyPanelNoticeStaffSaved:     "Ранг изменён. Перезапустите сервер, чтобы применить его.",

		KeyPanelErrorDoctorBlocked:        "doctor нашёл ошибку; мутация заблокирована.",
		KeyPanelErrorDoctorBlockedDetails: "doctor нашёл ошибку, мутация заблокирована: %s",
		KeyPanelErrorOpsUnavailable:       "ops не настроен: управление сервером недоступно.",
		KeyPanelErrorInternal:             "Внутренняя ошибка. Подробности в журнале панели.",
		KeyPanelErrorInternalShort:        "Внутренняя ошибка.",
		KeyPanelErrorInvalidArgument:      "Проверьте имя или значение.",
		KeyPanelErrorNotFound:             "Объект не найден.",
		KeyPanelErrorLocked:               "Другая операция уже выполняется. Попробуйте позже.",
		KeyPanelErrorValidation:           "Данные не прошли проверку.",
		KeyPanelErrorNoSpace:              "Недостаточно свободного места.",
		KeyPanelErrorServiceUnavailable:   "systemd, journald или unit недоступен.",
		KeyPanelErrorBadForm:              "Некорректная форма.",

		KeyPanelAuthRateLimited:       "Слишком много попыток входа. Подождите и попробуйте снова.",
		KeyPanelAuthBadForm:           "Некорректная форма входа.",
		KeyPanelAuthStaleForm:         "Форма устарела. Обновите страницу и попробуйте снова.",
		KeyPanelAuthPasswordRequired:  "Введите пароль.",
		KeyPanelAuthPasswordInvalid:   "Неверный пароль.",
		KeyPanelAuthRateLimitedChange: "Слишком много попыток. Подождите и попробуйте снова.",
		KeyPanelAuthBadFormChange:     "Некорректная форма.",
		KeyPanelAuthPasswordMismatch:  "Пароли не совпадают.",
		KeyPanelAuthPasswordSame:      "Новый пароль должен отличаться от текущего.",
		KeyPanelAuthPasswordShort:     "Новый пароль должен содержать не менее 12 символов.",
		KeyPanelAuthPasswordLong:      "Новый пароль слишком длинный.",
		KeyPanelAuthPasswordWrong:     "Текущий пароль неверен.",
		KeyPanelAuthPasswordSaveFail:  "Не удалось сохранить пароль.",

		KeyPanelPublicRateLimited: "Слишком много запросов. Попробуйте позже.",

		KeyPanelPublicBadRequest:      "Проверьте параметры запроса.",
		KeyPanelPublicNotFound:        "Запрошенный архив не найден.",
		KeyPanelPublicCorrupted:       "Архив или конфигурация повреждены.",
		KeyPanelPublicNoSpace:         "Недостаточно свободного места для операции.",
		KeyPanelPublicUnavailable:     "Сервер временно недоступен.",
		KeyPanelPublicUnavailableLong: "Сервер временно недоступен: systemd или unit не отвечает.",

		KeyPanelAdminWindowInvalid:     "Параметр window должен быть одним из: 1h, 6h, 24h.",
		KeyPanelAdminOpsUnavailable:    "ops не настроен: управление сервером недоступно.",
		KeyPanelAdminModsMissing:       "Текущий архив модов не найден.",
		KeyPanelAdminMetricsStopped:    "Сбор метрик не запущен.",
		KeyPanelAdminMetricsWindow:     "Неизвестное окно метрик.",
		KeyPanelAdminEventsUnavailable: "Последние события недоступны.",
		KeyPanelAdminTailInvalid:       "Параметр tail должен быть числом от 1 до 2000.",
		KeyPanelAdminArchiveInvalid:    "Некорректное имя архива.",
		KeyPanelAdminNoPrevious:        "Нет предыдущего архива для отката.",
		KeyPanelAdminPreviousInvalid:   "Предыдущий архив имеет неверное имя.",
		KeyPanelAdminFileMissing:       "В форме нет файла архива.",
		KeyPanelAdminMultipartExpected: "Ожидалась multipart-форма с архивом.",
		KeyPanelAdminModsTooLarge:      "Архив больше 512 MiB.",
		KeyPanelAdminStateTooLarge:     "Архив больше 1 ГиБ.",
		KeyPanelAdminUploadFailed:      "Не удалось сохранить загруженный файл.",
		KeyPanelAdminLimitInvalid:      "Параметр limit должен быть числом от 1 до 500.",
		KeyPanelAdminBeforeInvalid:     "Некорректное имя архива в параметре before.",
		KeyPanelAdminBackupUnavailable: "ops не настроен: создание бэкапа недоступно.",
		KeyPanelAdminRestoreUnavail:    "ops не настроен: восстановление недоступно.",
		KeyPanelAdminModesUnsupported:  "ops не умеет импортировать state и моды.",
		KeyPanelAdminStateNameInvalid:  "Некорректное имя state-архива.",
		KeyPanelAdminSavedFields:       "Сохранено полей: %d.",
		KeyPanelAdminPlayersUnreadable: "Не удалось прочитать список игроков.",
		KeyPanelAdminPlayerInvalid:     "Некорректное имя игрока.",
		KeyPanelAdminRankInvalid:       "Ранг должен быть числом от 0 до 3.",
		KeyPanelAdminFieldInvalid:      "Проверьте значение поля %s.",
		KeyPanelAdminModsNameInvalid:   "Некорректное имя архива модов.",
		KeyPanelAdminModsImportUnavail: "ops не настроен: импорт модов недоступен.",

		KeyPanelStatusRunning:     "работает",
		KeyPanelStatusStopped:     "остановлен",
		KeyPanelStatusStarting:    "запускается",
		KeyPanelStatusUnhealthy:   "нездоров",
		KeyPanelStatusUnavailable: "недоступен",
		KeyPanelStatusUnknown:     "неизвестно",

		KeyPanelHealthOK:       "в порядке",
		KeyPanelHealthProblems: "проблемы",
		KeyPanelHealthUnknown:  "неизвестно",

		KeyPanelGroupToday:     "Сегодня",
		KeyPanelGroupYesterday: "Вчера",
		KeyPanelGroupOther:     "Другие",
		KeyPanelGroupWeek:      "Неделя %d, %d",

		KeyPanelMonthJanuary:   "Январь",
		KeyPanelMonthFebruary:  "Февраль",
		KeyPanelMonthMarch:     "Март",
		KeyPanelMonthApril:     "Апрель",
		KeyPanelMonthMay:       "Май",
		KeyPanelMonthJune:      "Июнь",
		KeyPanelMonthJuly:      "Июль",
		KeyPanelMonthAugust:    "Август",
		KeyPanelMonthSeptember: "Сентябрь",
		KeyPanelMonthOctober:   "Октябрь",
		KeyPanelMonthNovember:  "Ноябрь",
		KeyPanelMonthDecember:  "Декабрь",

		KeyPanelTimeJustNow:              "только что",
		KeyPanelTimeMinuteAgo + ".one":   "%d минуту назад",
		KeyPanelTimeMinuteAgo + ".few":   "%d минуты назад",
		KeyPanelTimeMinuteAgo + ".many":  "%d минут назад",
		KeyPanelTimeHourAgo + ".one":     "%d час назад",
		KeyPanelTimeHourAgo + ".few":     "%d часа назад",
		KeyPanelTimeHourAgo + ".many":    "%d часов назад",
		KeyPanelTimeDayAgo + ".one":      "%d день назад",
		KeyPanelTimeDayAgo + ".few":      "%d дня назад",
		KeyPanelTimeDayAgo + ".many":     "%d дней назад",
		KeyPanelTimeMonthAgo + ".one":    "%d месяц назад",
		KeyPanelTimeMonthAgo + ".few":    "%d месяца назад",
		KeyPanelTimeMonthAgo + ".many":   "%d месяцев назад",
		KeyPanelTimeYearAgo + ".one":     "%d год назад",
		KeyPanelTimeYearAgo + ".few":     "%d года назад",
		KeyPanelTimeYearAgo + ".many":    "%d лет назад",
		KeyPanelTimeYearAgo + ".other":   "%d лет назад",
		KeyPanelTimeMinuteAgo + ".other": "%d минут назад",
		KeyPanelTimeHourAgo + ".other":   "%d часов назад",
		KeyPanelTimeDayAgo + ".other":    "%d дней назад",
		KeyPanelTimeMonthAgo + ".other":  "%d месяцев назад",

		KeyPanelRestoreUnsupported:      "Поддерживаются ZIP «состояние + моды» и state .tar.gz.",
		KeyPanelRestoreReadFailed:       "Не удалось прочитать загруженный файл.",
		KeyPanelRestoreEmpty:            "Загруженный файл пуст или повреждён.",
		KeyPanelRestoreNameFailed:       "Не удалось выбрать имя для импортируемого состояния.",
		KeyPanelRestoreZipUnreadable:    "ZIP не читается: %v",
		KeyPanelRestoreZipTwoFiles:      "ZIP должен содержать ровно два файла: state и моды.",
		KeyPanelRestoreZipRootOnly:      "ZIP должен содержать только два архива в корне.",
		KeyPanelRestoreZipTwoStates:     "В ZIP больше одного state-архива.",
		KeyPanelRestoreZipTwoMods:       "В ZIP больше одного архива модов.",
		KeyPanelRestoreZipUnexpected:    "Неожиданный файл в ZIP: %s",
		KeyPanelRestoreZipStateAndMods:  "ZIP должен содержать state-архив и архив модов.",
		KeyPanelRestoreTempDir:          "Не удалось подготовить временный каталог.",
		KeyPanelRestoreUnzipFailed:      "ZIP не распакован: %v",
		KeyPanelRestoreZipMissing:       "ZIP не содержит ожидаемый файл %s.",
		KeyPanelRestorePermissions:      "Не удалось выставить права на распакованный архив.",
		KeyPanelRestoreStateCheck:       "State-архив не прошёл проверку: %v",
		KeyPanelRestoreStateRead:        "Не удалось прочитать state-архив.",
		KeyPanelRestoreModsCheck:        "Архив модов не прошёл проверку: %v",
		KeyPanelRestoreModsRead:         "Не удалось прочитать архив модов.",
		KeyPanelRestoreModsMismatch:     "Архив модов не соответствует состоянию: state ожидает %s (%s), в ZIP лежит %s (%s).",
		KeyPanelRestoreDeployUnexpected: "ops задеплоил неожиданный архив модов %q.",
		KeyPanelRestoreModsMismatchLive: "Текущий пакет модов не совпадает с состоянием: %v",
		KeyPanelRestoreImportMods:       "ops импортировал неожиданный архив модов %q.",
		KeyPanelRestoreImportState:      "ops импортировал неожиданное состояние %q.",
		KeyPanelRestoreFileCheck:        "Не удалось проверить %s.",
		KeyPanelRestoreNotRegular:       "%s уже существует и не является обычным файлом.",
		KeyPanelRestoreFileRead:         "Не удалось прочитать %s.",
		KeyPanelRestoreExistsDiffers:    "%s уже существует с другим содержимым; переименуйте архив в ZIP или удалите старый файл вручную.",
	},
}

func init() {
	for language, strings := range panelCatalogs {
		catalog := catalogs[language]
		if catalog == nil {
			catalog = make(map[string]string, len(strings))
			catalogs[language] = catalog
		}
		for key, value := range strings {
			catalog[key] = value
		}
	}
}
