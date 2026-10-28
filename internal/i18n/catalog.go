package i18n

// Stable translation keys used by the UI.
const (
	KeyAppTitle   = "app.title"
	KeyAppTagline = "app.tagline"

	KeyCommonAdd       = "common.add"
	KeyCommonEdit      = "common.edit"
	KeyCommonDelete    = "common.delete"
	KeyCommonSave      = "common.save"
	KeyCommonCancel    = "common.cancel"
	KeyCommonClose     = "common.close"
	KeyCommonAbout     = "common.about"
	KeyCommonPlay      = "common.play"
	KeyCommonError     = "common.error"
	KeyCommonSetup     = "common.setup"
	KeyCommonAdvanced  = "common.advanced"
	KeyCommonEnterPath = "common.enter_path"

	KeyRailPlay        = "rail.play"
	KeyRailInstall     = "rail.installation"
	KeyRailOpenMWCS    = "rail.openmw_cs"
	KeyRailDiagnostics = "rail.diagnostics"
	KeyRailAbout       = "rail.about"

	KeyDiagnosticsTitle           = "diagnostics.title"
	KeyDiagnosticsIntro           = "diagnostics.intro"
	KeyDiagnosticsReport          = "diagnostics.report"
	KeyDiagnosticsReportHint      = "diagnostics.report.hint"
	KeyDiagnosticsReportPasswords = "diagnostics.report.passwords"
	KeyDiagnosticsReportLocal     = "diagnostics.report.local"
	KeyDiagnosticsCopyReport      = "diagnostics.copy_report"
	KeyDiagnosticsReportCopied    = "diagnostics.report_copied"
	KeyDiagnosticsFolders         = "diagnostics.folders"
	KeyDiagnosticsFoldersHint     = "diagnostics.folders.hint"
	KeyDiagnosticsFolderWorkDir   = "diagnostics.folders.work_dir"
	KeyDiagnosticsFolderTES3MP    = "diagnostics.folders.tes3mp"
	KeyDiagnosticsFolderOpenMW    = "diagnostics.folders.openmw"
	KeyDiagnosticsFolderLog       = "diagnostics.folders.log"

	KeyInstallTitle            = "install.title"
	KeyInstallIntro            = "install.intro"
	KeyInstallRunWizard        = "install.run_wizard"
	KeyInstallMorrowind        = "install.morrowind"
	KeyInstallTES3MP           = "install.tes3mp"
	KeyInstallMorrowindMissing = "install.morrowind.missing"
	KeyInstallChangeFolder     = "install.change_folder"
	KeyInstallReinstall        = "install.reinstall"
	KeyInstallFonts            = "install.fonts"
	KeyInstallFontsReady       = "install.fonts.ready"
	KeyInstallFontsReadyDetail = "install.fonts.ready_detail"
	KeyInstallFontsPartial     = "install.fonts.partial"
	KeyInstallFontsMissing     = "install.fonts.missing"
	KeyInstallFontsMissingSet  = "install.fonts.missing_set"
	KeyInstallFontsInstall     = "install.fonts.install"
	KeyFontsSet                = "fonts.set"
	KeyFontsSetHint            = "fonts.set.hint"
	KeyFontsSetLicense         = "fonts.set.license"
	KeyFontsSetDefault         = "fonts.set.default"
	KeyFontsSetChange          = "fonts.set.change"
	KeyFontsSetApply           = "fonts.set.apply"
	KeyFontsSetRequired        = "fonts.set.required"
	KeyFontsDownloading        = "fonts.downloading"
	KeyFontsDownloadingAsset   = "fonts.downloading_asset"
	KeyFontsDownloadingBytes   = "fonts.downloading_bytes"
	KeyFontsInstalling         = "fonts.installing"
	KeyFontsVerifying          = "fonts.verifying"
	KeyFontsCanceled           = "fonts.canceled"
	KeyFontsDownloadFailed     = "fonts.download_failed"
	KeyFontsCancel             = "fonts.cancel"
	KeyFontsRetry              = "fonts.retry"
	KeyInstallLanguage         = "install.language"
	KeyInstallLanguageOriginal = "install.language.original"
	KeyInstallLanguageText     = "install.language.text"
	KeyInstallLanguageVoices   = "install.language.voices"
	KeyInstallInstall          = "install.install"
	KeyInstallChange           = "install.change"
	KeyInstallBrowse           = "install.browse"
	KeyInstallWorkDirHint      = "install.work_dir_hint"
	KeyInstallProtonFound      = "install.proton_found"
	KeyInstallProtonManual     = "install.proton_manual"
	KeyInstallProtonMissing    = "install.proton_missing"

	KeyUninstall              = "uninstall"
	KeyUninstallDangerZone    = "uninstall.danger_zone"
	KeyUninstallTitle         = "uninstall.title"
	KeyUninstallHint          = "uninstall.hint"
	KeyUninstallGameFiles     = "uninstall.game_files"
	KeyUninstallCache         = "uninstall.cache"
	KeyUninstallSettings      = "uninstall.settings"
	KeyUninstallKeepMorrowind = "uninstall.keep_morrowind"
	KeyUninstallEmpty         = "uninstall.empty"
	KeyUninstallRunning       = "uninstall.running"
	KeyUninstallDone          = "uninstall.done"
	KeyUninstallRestart       = "uninstall.restart"
	KeyUninstallRemove        = "uninstall.remove"

	KeyInstallUIVideo              = "install.uivideo"
	KeyInstallUIVideoConfigure     = "install.uivideo.configure"
	KeyInstallUIVideoReady         = "install.uivideo.ready"
	KeyInstallUIVideoNoFile        = "install.uivideo.no_file"
	KeyInstallUIVideoDefaults      = "install.uivideo.defaults"
	KeyInstallUIVideoTitle         = "install.uivideo.title"
	KeyInstallUIVideoHint          = "install.uivideo.hint"
	KeyInstallUIVideoTTF           = "install.uivideo.ttf"
	KeyInstallUIVideoTTFHint       = "install.uivideo.ttf_hint"
	KeyInstallUIVideoFontSize      = "install.uivideo.font_size"
	KeyInstallUIVideoFontSizeHint  = "install.uivideo.font_size_hint"
	KeyInstallUIVideoScaling       = "install.uivideo.scaling"
	KeyInstallUIVideoScalingHint   = "install.uivideo.scaling_hint"
	KeyInstallUIVideoFramerate     = "install.uivideo.framerate"
	KeyInstallUIVideoFramerateHint = "install.uivideo.framerate_hint"
	KeyInstallUIVideoInvalid       = "install.uivideo.invalid"
	KeyInstallUIVideoUnreadable    = "install.uivideo.unreadable"
	KeyInstallUIVideoSaved         = "install.uivideo.saved"

	KeyCSHeading      = "cs.title"
	KeyCSEditor       = "cs.editor"
	KeyCSIntro        = "cs.intro"
	KeyCSFound        = "cs.found"
	KeyCSMissing      = "cs.missing"
	KeyCSExpected     = "cs.expected"
	KeyCSLaunch       = "cs.launch"
	KeyCSCheckAgain   = "cs.check_again"
	KeyCSOpenFolder   = "cs.open_folder"
	KeyCSModsRequired = "cs.mods_required"
	KeyCSNoServer     = "cs.no_server"
	KeyCSProtonNote   = "cs.proton_note"
	KeyCSStarted      = "cs.started"

	KeyServerEmptyTitle  = "server.empty.title"
	KeyServerEmptyHint   = "server.empty.hint"
	KeyServerListTitle   = "server.list.title"
	KeyServerListHint    = "server.list.hint"
	KeyServerActive      = "server.active"
	KeyServerPanel       = "server.panel"
	KeyServerAddTitle    = "server.add.title"
	KeyServerEditTitle   = "server.edit.title"
	KeyServerDeleteTitle = "server.delete.title"
	KeyServerDeleteBody  = "server.delete.body"
	KeyServerFieldName   = "server.field.name"
	KeyServerFieldAddr   = "server.field.address"
	KeyServerFieldPass   = "server.field.password"
	KeyServerFieldPanel  = "server.field.panel_url"
	KeyServerUpdateMods  = "server.update_mods"
	KeyServerLaunching   = "server.launching"
	KeyServerLaunched    = "server.launched"

	KeyServerAlreadyStarting = "server.already_starting"
	KeyServerAlreadyRunning  = "server.already_running"

	KeyServerMorrowindRequired = "server.morrowind_required"

	KeyServerMore            = "server.more"
	KeyServerActionsTitle    = "server.actions.title"
	KeyServerOpenPanel       = "server.open_panel"
	KeyServerPlayDownload    = "server.play_download"
	KeyServerPlayUpdate      = "server.play_update"
	KeyServerStatusOnline    = "server.status.online"
	KeyServerStatusUpdate    = "server.status.update"
	KeyServerStatusChecking  = "server.status.checking"
	KeyServerPreparing       = "server.preparing"
	KeyServerEndpointHint    = "server.field.endpoint_hint"
	KeyServerAdvancedOptions = "server.advanced_options"
	KeyServerPanelHint       = "server.field.panel_hint"

	KeyModsTitle       = "mods.title"
	KeyModsChecking    = "mods.checking"
	KeyModsDownloading = "mods.downloading"
	KeyModsVerifying   = "mods.verifying"
	KeyModsExtracting  = "mods.extracting"
	KeyModsInstalling  = "mods.installing"
	KeyModsUpToDate    = "mods.up_to_date"
	KeyModsDone        = "mods.done"

	KeySettingsWorkDir = "settings.work_dir"
	KeySettingsProton  = "settings.proton"
	KeySettingsTheme   = "settings.theme"

	KeyThemeSystem = "theme.system"
	KeyThemeDark   = "theme.dark"
	KeyThemeLight  = "theme.light"

	KeyLocaleTitle              = "locale.title"
	KeyLocaleQuestion           = "locale.question"
	KeyLocaleOnlyPack           = "locale.only_pack"
	KeyLocaleVariantText        = "locale.variant.text"
	KeyLocaleVariantTextHint    = "locale.variant.text_hint"
	KeyLocaleVariantVoices      = "locale.variant.voices"
	KeyLocaleVariantVoicesHint  = "locale.variant.voices_hint"
	KeyLocaleNoVoices           = "locale.no_voices"
	KeyLocaleInstalled          = "locale.installed"
	KeyLocaleInstall            = "locale.install"
	KeyLocaleChange             = "locale.change"
	KeyLocaleChecking           = "locale.checking"
	KeyLocaleDownloading        = "locale.downloading"
	KeyLocaleExtracting         = "locale.extracting"
	KeyLocaleInstalling         = "locale.installing"
	KeyLocaleRemoved            = "locale.removed"
	KeyLanguageWizardRemoveHint = "language.remove.hint"
	KeyLanguageWizardRemove     = "language.remove"
	KeyLanguageWizardApplying   = "language.applying"
	KeyLanguageWizardDone       = "language.done"

	KeyErrorWhat         = "error.what"
	KeyErrorDetail       = "error.detail"
	KeyErrorActionLine   = "error.action_line"
	KeyErrorLogLine      = "error.log_line"
	KeyErrorRetry        = "error.retry"
	KeyErrorUnknown      = "error.unknown"
	KeyErrorUnknownAct   = "error.unknown_action"
	KeyErrorCode         = "error.code"
	KeyErrorShowDetails  = "error.show_details"
	KeyErrorNoDetails    = "error.no_details"
	KeyErrorOpenLog      = "error.open_log"
	KeyErrorCopyReport   = "error.copy_report"
	KeyErrorReportCopied = "error.report_copied"

	KeyErrorMorrowindNotFound      = "error.morrowind_not_found"
	KeyErrorMorrowindNotFoundAct   = "error.morrowind_not_found_action"
	KeyErrorMorrowindIncomplete    = "error.morrowind_incomplete"
	KeyErrorMorrowindIncompleteAct = "error.morrowind_incomplete_action"
	KeyErrorMorrowindMissing       = "error.morrowind_not_configured"
	KeyErrorMorrowindMissingAct    = "error.morrowind_not_configured_action"
	KeyErrorNetwork                = "error.network"
	KeyErrorNetworkAct             = "error.network_action"
	KeyErrorTimeout                = "error.timeout"
	KeyErrorTimeoutAct             = "error.timeout_action"
	KeyErrorCertificate            = "error.certificate"
	KeyErrorCertificateAct         = "error.certificate_action"
	KeyErrorNoMods                 = "error.no_mods"
	KeyErrorNoModsAct              = "error.no_mods_action"
	KeyErrorServerAPI              = "error.server_api"
	KeyErrorServerAPIAct           = "error.server_api_action"
	KeyErrorFileSystem             = "error.file_system"
	KeyErrorFileSystemAct          = "error.file_system_action"
	KeyErrorWorkDir                = "error.work_dir"
	KeyErrorWorkDirAct             = "error.work_dir_action"
	KeyErrorModsMissing            = "error.mods_missing"
	KeyErrorModsMissingAct         = "error.mods_missing_action"
	KeyErrorLocaleMissing          = "error.locale_missing"
	KeyErrorLocaleMissingAct       = "error.locale_missing_action"
	KeyErrorGameMissing            = "error.game_missing"
	KeyErrorGameMissingAct         = "error.game_missing_action"
	KeyErrorGameProton             = "error.game_proton"
	KeyErrorGameProtonAct          = "error.game_proton_action"
	KeyErrorGameConfig             = "error.game_config"
	KeyErrorGameConfigAct          = "error.game_config_action"
	KeyErrorGameStart              = "error.game_start"
	KeyErrorGameStartAct           = "error.game_start_action"
	KeyErrorGameExited             = "error.game_exited"
	KeyErrorGameExitedAct          = "error.game_exited_action"
	KeyErrorGameStarting           = "error.game_starting"
	KeyErrorGameStartingAct        = "error.game_starting_action"
	KeyErrorGameRunning            = "error.game_running"
	KeyErrorGameRunningAct         = "error.game_running_action"

	KeyLogsTitle      = "logs.title"
	KeyLogsEmpty      = "logs.empty"
	KeyLogsRefresh    = "logs.refresh"
	KeyLogsOpenFolder = "logs.open_folder"
	KeyLogsHint       = "logs.hint"

	KeyWizardTitle          = "wizard.title"
	KeyWizardStep           = "wizard.step"
	KeyWizardCancel         = "wizard.cancel"
	KeyWizardBack           = "wizard.back"
	KeyWizardNext           = "wizard.next"
	KeyWizardFinish         = "wizard.finish"
	KeyWizardRetry          = "wizard.retry"
	KeyWizardMorrowindTitle = "wizard.morrowind.title"
	KeyWizardMorrowindHint  = "wizard.morrowind.hint"
	KeyWizardFontsTitle     = "wizard.fonts.title"
	KeyWizardFontsHint      = "wizard.fonts.hint"
	KeyWizardFontsPending   = "wizard.fonts.pending"
	KeyWizardFontsPartial   = "wizard.fonts.partial"
	KeyFontsPreviewHint     = "fonts.set.preview_hint"
	KeyWizardTES3MPTitle    = "wizard.tes3mp.title"
	KeyWizardTES3MPHint     = "wizard.tes3mp.hint"

	KeySetupFind                = "setup.find"
	KeySetupChoose              = "setup.choose"
	KeySetupCheck               = "setup.check"
	KeySetupSearching           = "setup.searching"
	KeySetupChecking            = "setup.checking"
	KeySetupMorrowindReady      = "setup.morrowind.ready"
	KeySetupMorrowindIncomplete = "setup.morrowind.incomplete"
	KeySetupMorrowindNotFound   = "setup.morrowind.not_found"
	KeySetupTES3MP              = "setup.tes3mp"
	KeySetupTES3MPReady         = "setup.tes3mp.ready"
	KeySetupTES3MPOutdated      = "setup.tes3mp.outdated"
	KeySetupTES3MPMissing       = "setup.tes3mp.missing"
	KeySetupInstall             = "setup.install"
	KeySetupDownloading         = "setup.downloading"
	KeySetupStarting            = "setup.starting"
	KeySetupVerifying           = "setup.verifying"
	KeySetupExtracting          = "setup.extracting"
	KeySetupInstalling          = "setup.installing"
	KeySetupDone                = "setup.done"

	KeyAboutTitle        = "about.title"
	KeyAboutIntro        = "about.intro"
	KeyAboutVersion      = "about.version"
	KeyAboutHint         = "about.hint"
	KeyAboutLicense      = "about.license"
	KeyAboutLicenseValue = "about.license.value"
	KeyAboutOpenSource   = "about.open_source"
	KeyAboutProject      = "about.project"
	KeyAboutOpenPage     = "about.open_page"
	KeyAboutComponents   = "about.components"
	KeyAboutTES3MP       = "about.components.tes3mp"
	KeyAboutTES3MPHint   = "about.components.tes3mp.hint"
	KeyAboutOpenMW       = "about.components.openmw"
	KeyAboutOpenMWHint   = "about.components.openmw.hint"
	KeyAboutFyne         = "about.components.fyne"
	KeyAboutFyneHint     = "about.components.fyne.hint"
	KeyAboutNotInstalled = "about.not_installed"
	KeyAboutReady        = "about.ready"
	KeyAboutSystem       = "about.system"
	KeyAboutLegal        = "about.legal"
	KeyFooterPath        = "footer.config_path"
	KeyFooterVersion     = "footer.version"

	KeyLanguageRussian = "lang.russian"
)

var catalogs = map[Language]map[string]string{
	English: {
		KeyAppTitle:   "tes3mp-easy",
		KeyAppTagline: "Morrowind with friends",

		KeyCommonAdd:       "Add",
		KeyCommonEdit:      "Edit",
		KeyCommonDelete:    "Delete",
		KeyCommonSave:      "Save",
		KeyCommonCancel:    "Cancel",
		KeyCommonClose:     "Close",
		KeyCommonAbout:     "About",
		KeyCommonPlay:      "Play",
		KeyCommonError:     "Error",
		KeyCommonSetup:     "Setup",
		KeyCommonAdvanced:  "Advanced",
		KeyCommonEnterPath: "Enter path…",

		KeyRailPlay:        "Play",
		KeyRailInstall:     "Installation",
		KeyRailOpenMWCS:    "OpenMW-CS",
		KeyRailDiagnostics: "Diagnostics",
		KeyRailAbout:       "About",

		KeyDiagnosticsTitle:           "Diagnostics",
		KeyDiagnosticsIntro:           "What the client checks before every launch: paths, game files, fonts and configuration.",
		KeyDiagnosticsReport:          "Diagnostic report",
		KeyDiagnosticsReportHint:      "Paths, versions, settings and the last launch errors in one text file.",
		KeyDiagnosticsReportPasswords: "Server passwords are never included.",
		KeyDiagnosticsReportLocal:     "The report is stored only locally and is not sent anywhere by the client.",
		KeyDiagnosticsCopyReport:      "Copy report",
		KeyDiagnosticsReportCopied:    "The diagnostic report was copied to the clipboard.",
		KeyDiagnosticsFolders:         "Folders",
		KeyDiagnosticsFoldersHint:     "Paths open in the system file manager.",
		KeyDiagnosticsFolderWorkDir:   "Work folder",
		KeyDiagnosticsFolderTES3MP:    "TES3MP",
		KeyDiagnosticsFolderOpenMW:    "OpenMW config",
		KeyDiagnosticsFolderLog:       "Client log",

		KeyInstallTitle:                "Installation",
		KeyInstallIntro:                "Everything the client needs is checked here. Paths are automatic and the Morrowind folder is only read.",
		KeyInstallRunWizard:            "Run wizard",
		KeyInstallMorrowind:            "Morrowind",
		KeyInstallTES3MP:               "TES3MP",
		KeyInstallMorrowindMissing:     "Morrowind folder is not selected yet.",
		KeyInstallChangeFolder:         "Change folder",
		KeyInstallReinstall:            "Reinstall",
		KeyInstallFonts:                "Fonts",
		KeyInstallFontsReady:           "The %s font set is installed.",
		KeyInstallFontsReadyDetail:     "The %s font set is installed in %s.",
		KeyInstallFontsPartial:         "The font set is incomplete, missing: %s",
		KeyInstallFontsMissing:         "Fonts are not installed yet.",
		KeyInstallFontsMissingSet:      "The %s font set is not installed yet.",
		KeyInstallFontsInstall:         "Install fonts and continue",
		KeyFontsSet:                    "Font set",
		KeyFontsSetHint:                "The interface, Daedric slots and console font. Every set covers Latin and Cyrillic; the download is verified and does not block the game.",
		KeyFontsSetLicense:             "All sets are OFL-licensed Google Fonts and are downloaded from the official repository.",
		KeyFontsSetDefault:             "(default)",
		KeyFontsSetChange:              "Change set",
		KeyFontsSetApply:               "Apply and install",
		KeyFontsSetRequired:            "Choose a font set first.",
		KeyFontsDownloading:            "Downloading the font set…",
		KeyFontsDownloadingAsset:       "Downloading %s…",
		KeyFontsDownloadingBytes:       "Downloading %s… %s of %s",
		KeyFontsInstalling:             "Installing %s…",
		KeyFontsVerifying:              "Verifying the font set…",
		KeyFontsCanceled:               "Font download canceled.",
		KeyFontsDownloadFailed:         "The fonts could not be downloaded.",
		KeyFontsCancel:                 "Cancel",
		KeyFontsRetry:                  "Retry",
		KeyInstallLanguage:             "Game language",
		KeyInstallLanguageOriginal:     "No pack installed — the game keeps the original Morrowind language.",
		KeyInstallLanguageText:         "%s · text",
		KeyInstallLanguageVoices:       "%s · text + voice",
		KeyInstallInstall:              "Install",
		KeyInstallChange:               "Change…",
		KeyInstallBrowse:               "Browse…",
		KeyInstallWorkDirHint:          "Keep the default. Changing the folder after installation requires installing TES3MP and the mods again.",
		KeyInstallProtonFound:          "Found automatically",
		KeyInstallProtonManual:         "Set manually",
		KeyInstallProtonMissing:        "Not found — choose the folder",
		KeyUninstall:                   "Uninstall",
		KeyUninstallDangerZone:         "Danger zone",
		KeyUninstallTitle:              "Uninstall TES3MP Easy",
		KeyUninstallHint:               "Selected folders are deleted from disk and cannot be restored. The Morrowind installation is never modified.",
		KeyUninstallGameFiles:          "Game files: TES3MP, servers, mods, fonts and localization",
		KeyUninstallCache:              "Downloaded archives (cache)",
		KeyUninstallSettings:           "Client settings and server profiles",
		KeyUninstallKeepMorrowind:      "Morrowind, its Data Files and the game's own settings stay untouched.",
		KeyUninstallEmpty:              "Select at least one item to remove.",
		KeyUninstallRunning:            "Removing files…",
		KeyUninstallDone:               "Removed:\n%s",
		KeyUninstallRestart:            "Client settings were removed. The launcher closes now and the next start begins with a fresh setup.",
		KeyUninstallRemove:             "Remove",
		KeyInstallUIVideo:              "UI and video",
		KeyInstallUIVideoConfigure:     "Configure",
		KeyInstallUIVideoReady:         "Configured: settings.cfg keeps the launcher values.",
		KeyInstallUIVideoNoFile:        "settings.cfg appears after the first launch; you can configure it now.",
		KeyInstallUIVideoDefaults:      "Not configured yet: OpenMW defaults are used.",
		KeyInstallUIVideoTitle:         "UI and video",
		KeyInstallUIVideoHint:          "Only the four values below are changed. Comments, other keys and line endings in settings.cfg stay as they are; changes apply to the next launch.",
		KeyInstallUIVideoTTF:           "TTF resolution",
		KeyInstallUIVideoTTFHint:       "Glyph atlas size: higher means crisper letters at a large font size and more video memory. This is what removes the blurry stock text.",
		KeyInstallUIVideoFontSize:      "Font size",
		KeyInstallUIVideoFontSizeHint:  "Base interface text size. A large font on a small atlas looks rough — raise both together.",
		KeyInstallUIVideoScaling:       "Scaling factor",
		KeyInstallUIVideoScalingHint:   "Scale of menus and HUD: 1.00 is the original size, 1.50–2.00 is comfortable on 1440p and 4K.",
		KeyInstallUIVideoFramerate:     "Framerate limit",
		KeyInstallUIVideoFramerateHint: "0 means no limit. 60 is enough for Morrowind; on a 120–144 Hz display use 120 or 144.",
		KeyInstallUIVideoInvalid:       "Enter a value inside the range shown next to each label.",
		KeyInstallUIVideoUnreadable:    "settings.cfg cannot be read: %s",
		KeyInstallUIVideoSaved:         "settings.cfg is updated.",

		KeyCSHeading:      "OpenMW-CS",
		KeyCSEditor:       "Editor",
		KeyCSIntro:        "OpenMW Construction Set opens the same Morrowind data and the active server's mods as the game.",
		KeyCSFound:        "The editor is installed: %s",
		KeyCSMissing:      "The editor is missing",
		KeyCSExpected:     "Expected: %s",
		KeyCSLaunch:       "Launch editor",
		KeyCSCheckAgain:   "Check again",
		KeyCSOpenFolder:   "Open folder",
		KeyCSModsRequired: "Update the mods of the active server before opening the editor.",
		KeyCSNoServer:     "Add a server first: the editor opens its mods.",
		KeyCSProtonNote:   "On Linux the editor starts in the same Proton prefix as the game, otherwise it cannot see Documents/My Games/OpenMW.",
		KeyCSStarted:      "OpenMW-CS is running.",

		KeyServerListTitle:   "Servers",
		KeyServerListHint:    "One card per server. Play downloads missing mods and writes the endpoint.",
		KeyServerEmptyTitle:  "No servers yet",
		KeyServerEmptyHint:   "Add the server address your friends sent you.",
		KeyServerActive:      "active",
		KeyServerPanel:       "Panel",
		KeyServerAddTitle:    "Add server",
		KeyServerEditTitle:   "Edit server",
		KeyServerDeleteTitle: "Delete server",
		KeyServerDeleteBody:  "Delete the profile %q? Files already downloaded stay on disk.",
		KeyServerFieldName:   "Name",
		KeyServerFieldAddr:   "Address",
		KeyServerFieldPass:   "Password",
		KeyServerFieldPanel:  "Panel URL",
		KeyServerUpdateMods:  "Update mods",
		KeyServerLaunching:   "Preparing the launch...",
		KeyServerLaunched:    "The game is running.",

		KeyServerAlreadyStarting: "TES3MP is already starting. Wait until the game window appears.",
		KeyServerAlreadyRunning:  "TES3MP is already running. Close the game before starting it again.",

		KeyServerMorrowindRequired: "Choose the Morrowind folder on the Installation screen to play.",

		KeyServerMore:            "More",
		KeyServerActionsTitle:    "Server actions",
		KeyServerOpenPanel:       "Open panel",
		KeyServerPlayDownload:    "Download mods and play",
		KeyServerPlayUpdate:      "Update and play",
		KeyServerStatusOnline:    "Online · %s",
		KeyServerStatusUpdate:    "mods update available",
		KeyServerStatusChecking:  "Checking the server...",
		KeyServerPreparing:       "Preparing the launch...",
		KeyServerEndpointHint:    "Paste a host, host:port, or the panel's https:// address.",
		KeyServerAdvancedOptions: "Advanced options",
		KeyServerPanelHint:       "Leave empty to use https://<address> for the public panel.",

		KeyModsTitle:       "Server mods",
		KeyModsChecking:    "Checking server mods...",
		KeyModsDownloading: "Downloading mods: %s of %s",
		KeyModsVerifying:   "Checking the mods archive...",
		KeyModsExtracting:  "Extracting mods...",
		KeyModsInstalling:  "Installing mods...",
		KeyModsUpToDate:    "Mods are up to date.",
		KeyModsDone:        "Mods are installed.",

		KeySettingsTheme:   "Theme",
		KeySettingsWorkDir: "Work folder",
		KeySettingsProton:  "Proton (Linux)",

		KeyThemeSystem: "System",
		KeyThemeDark:   "Dark",
		KeyThemeLight:  "Light",

		KeyLocaleTitle:              "Game language",
		KeyLocaleQuestion:           "A language pack replaces the game text and, optionally, the voices. Without a pack the game runs in its original language — that is a normal setup, not an error.",
		KeyLocaleOnlyPack:           "Only pack in the catalog",
		KeyLocaleVariantText:        "Text only",
		KeyLocaleVariantTextHint:    "≈%s · interface text and dialogue subtitles · encoding %s",
		KeyLocaleVariantVoices:      "Text and voiceover",
		KeyLocaleVariantVoicesHint:  "≈%s · adds the dubbed dialogue audio · the download takes longer",
		KeyLocaleNoVoices:           "This pack has no voiceover; only the text is installed.",
		KeyLocaleInstalled:          "Installed: %s",
		KeyLocaleInstall:            "Install language pack…",
		KeyLocaleChange:             "Change…",
		KeyLocaleChecking:           "Checking the installed language files...",
		KeyLocaleDownloading:        "Downloading language files: %s of %s",
		KeyLocaleExtracting:         "Extracting language files...",
		KeyLocaleInstalling:         "Applying language files...",
		KeyLocaleRemoved:            "The localization overlay was removed.",
		KeyLanguageWizardRemoveHint: "The localization overlay will be removed; every profile returns to the original Morrowind language.",
		KeyLanguageWizardRemove:     "Remove",
		KeyLanguageWizardApplying:   "Installing localization...",
		KeyLanguageWizardDone:       "Localization is installed for all server profiles.",

		KeyErrorWhat:         "What happened: %s",
		KeyErrorDetail:       "Technical details: %s",
		KeyErrorActionLine:   "What to do: %s",
		KeyErrorLogLine:      "Log: %s",
		KeyErrorRetry:        "Retry",
		KeyErrorUnknown:      "The operation failed.",
		KeyErrorUnknownAct:   "Open the log, check the details and try again.",
		KeyErrorCode:         "Support code: %s",
		KeyErrorShowDetails:  "Technical details",
		KeyErrorNoDetails:    "No technical details are available.",
		KeyErrorOpenLog:      "Open log",
		KeyErrorCopyReport:   "Copy report",
		KeyErrorReportCopied: "The diagnostic report was copied without the server password.",

		KeyErrorMorrowindNotFound:      "Morrowind was not found.",
		KeyErrorMorrowindNotFoundAct:   "Select the folder that contains Data Files/Morrowind.esm, or install Morrowind first.",
		KeyErrorMorrowindIncomplete:    "The Morrowind folder is incomplete.",
		KeyErrorMorrowindIncompleteAct: "Select a folder with Data Files/Morrowind.esm, Tribunal.esm and Bloodmoon.esm.",
		KeyErrorMorrowindMissing:       "No Morrowind folder is selected.",
		KeyErrorMorrowindMissingAct:    "Choose the Morrowind folder in Setup or Settings.",
		KeyErrorNetwork:                "The server did not respond.",
		KeyErrorNetworkAct:             "Check the panel URL, the internet connection and the firewall, then retry.",
		KeyErrorTimeout:                "The server took too long to answer.",
		KeyErrorTimeoutAct:             "Retry in a moment. If it keeps timing out, check the server or your connection.",
		KeyErrorCertificate:            "The panel certificate could not be verified.",
		KeyErrorCertificateAct:         "Check the system date and the panel URL. Do not disable certificate checks.",
		KeyErrorNoMods:                 "The panel has no current mod pack.",
		KeyErrorNoModsAct:              "Ask the server owner to publish mods in the panel and try again.",
		KeyErrorServerAPI:              "The panel returned an error.",
		KeyErrorServerAPIAct:           "Check the panel URL and retry. If it repeats, send the log to the server owner.",
		KeyErrorFileSystem:             "A file could not be read or written.",
		KeyErrorFileSystemAct:          "Check free disk space and write permissions for the work folder, then retry.",
		KeyErrorWorkDir:                "The work folder is invalid.",
		KeyErrorWorkDirAct:             "Choose an existing writable folder with an absolute path in Setup or Settings.",
		KeyErrorModsMissing:            "Server mods are not installed.",
		KeyErrorModsMissingAct:         "Run Setup or press Update mods, then retry.",
		KeyErrorLocaleMissing:          "The installed localization is missing or damaged.",
		KeyErrorLocaleMissingAct:       "Open Diagnostics, find the client log in the Folders card and reinstall the language pack in Installation.",
		KeyErrorGameMissing:            "TES3MP is not installed.",
		KeyErrorGameMissingAct:         "Open Setup and install TES3MP, then retry.",
		KeyErrorGameProton:             "The Proton runtime is not usable.",
		KeyErrorGameProtonAct:          "Select a working Proton folder in Installation → Advanced, or install Proton through Steam.",
		KeyErrorGameConfig:             "The game configuration could not be prepared.",
		KeyErrorGameConfigAct:          "Check write permissions for the work folder and the OpenMW config folder, then retry.",
		KeyErrorGameStart:              "TES3MP could not be started.",
		KeyErrorGameStartAct:           "Check that TES3MP is installed correctly, close other instances and retry.",
		KeyErrorGameExited:             "TES3MP exited unexpectedly.",
		KeyErrorGameExitedAct:          "Open the client log for the game's error and retry after fixing it.",
		KeyErrorGameStarting:           "TES3MP is still starting.",
		KeyErrorGameStartingAct:        "Wait until the game window appears instead of launching again.",
		KeyErrorGameRunning:            "TES3MP is still running.",
		KeyErrorGameRunningAct:         "Close the game window and retry to start a fresh instance.",

		KeyLogsTitle:      "Client log",
		KeyLogsEmpty:      "The log is empty yet.",
		KeyLogsRefresh:    "Refresh",
		KeyLogsOpenFolder: "Open log folder",
		KeyLogsHint:       "The last %d lines are shown. The full file is in the log folder.",

		KeyWizardTitle:          "First-run setup",
		KeyWizardStep:           "Step %d of %d",
		KeyWizardCancel:         "Cancel setup",
		KeyWizardBack:           "Back",
		KeyWizardNext:           "Next",
		KeyWizardFinish:         "Finish",
		KeyWizardRetry:          "Retry",
		KeyWizardMorrowindTitle: "Morrowind",
		KeyWizardMorrowindHint:  "Select the original game folder. The client only reads it.",
		KeyWizardFontsTitle:     "Fonts and interface",
		KeyWizardFontsHint:      "The files replace the low-resolution stock fonts in My Games/OpenMW/Fonts and openmw.cfg gets the matching fallback= lines — text stops looking blurry at high resolution and Cyrillic stops showing as boxes. The set is checked before every launch; a failed download is retried on the next Play. The four values below are the same ones the Configure dialog writes to settings.cfg, with their range in the label.",
		KeyWizardFontsPending:   "The %s font set will be downloaded in the background after setup; Play is not blocked by it.",
		KeyWizardFontsPartial:   "Some fonts are missing (%s). They will be downloaded again in the background.",
		KeyFontsPreviewHint:     "The sample uses the installed file of the chosen set; before the download it falls back to the launcher font.",
		KeyWizardTES3MPTitle:    "TES3MP",
		KeyWizardTES3MPHint:     "The client downloads and verifies TES3MP %s. No server details are needed for this step.",

		KeySetupFind:                "Find automatically",
		KeySetupChoose:              "Choose folder",
		KeySetupCheck:               "Check files",
		KeySetupSearching:           "Looking for Morrowind...",
		KeySetupChecking:            "Checking the folder...",
		KeySetupMorrowindReady:      "Morrowind is ready: %s",
		KeySetupMorrowindIncomplete: "Cannot use this folder: %s",
		KeySetupMorrowindNotFound:   "Morrowind was not found. Choose the game folder, or continue without it.",
		KeySetupTES3MP:              "TES3MP %s",
		KeySetupTES3MPReady:         "TES3MP %s is installed.",
		KeySetupTES3MPOutdated:      "TES3MP %s is installed, but %s is required.",
		KeySetupTES3MPMissing:       "TES3MP is not installed yet.",
		KeySetupInstall:             "Install TES3MP",
		KeySetupDownloading:         "Downloading TES3MP: %s of %s",
		KeySetupStarting:            "Preparing the TES3MP download...",
		KeySetupVerifying:           "Checking the TES3MP archive...",
		KeySetupExtracting:          "Extracting TES3MP...",
		KeySetupInstalling:          "Finishing the TES3MP installation...",
		KeySetupDone:                "TES3MP is installed and ready.",

		KeyAboutTitle:        "About",
		KeyAboutIntro:        "A client that installs and launches TES3MP with the right fonts and settings, without editing files by hand.",
		KeyAboutVersion:      "Version %s",
		KeyAboutHint:         "How the client is built and what is installed.",
		KeyAboutLicense:      "License",
		KeyAboutLicenseValue: "MIT",
		KeyAboutOpenSource:   "Open source",
		KeyAboutProject:      "Project",
		KeyAboutOpenPage:     "Open project page",
		KeyAboutComponents:   "Components",
		KeyAboutTES3MP:       "TES3MP %s",
		KeyAboutTES3MPHint:   "Multiplayer fork of OpenMW used to play on servers.",
		KeyAboutOpenMW:       "OpenMW",
		KeyAboutOpenMWHint:   "Open-source engine the client configures and launches.",
		KeyAboutFyne:         "Fyne",
		KeyAboutFyneHint:     "Toolkit that draws this window.",
		KeyAboutNotInstalled: "Not installed",
		KeyAboutReady:        "Installed",
		KeyAboutSystem:       "System",
		KeyAboutLegal:        "Morrowind and its data files belong to Bethesda Softworks. The client never modifies the game folder.",
		KeyFooterPath:        "Config: %s",
		KeyFooterVersion:     "tes3mp-easy %s · TES3MP %s",

		KeyLanguageRussian: "Russian",
	},
	Russian: {
		KeyAppTitle:   "tes3mp-easy",
		KeyAppTagline: "Morrowind с друзьями",

		KeyCommonAdd:       "Добавить",
		KeyCommonEdit:      "Изменить",
		KeyCommonDelete:    "Удалить",
		KeyCommonSave:      "Сохранить",
		KeyCommonCancel:    "Отмена",
		KeyCommonClose:     "Закрыть",
		KeyCommonAbout:     "О клиенте",
		KeyCommonPlay:      "Играть",
		KeyCommonError:     "Ошибка",
		KeyCommonSetup:     "Настройка",
		KeyCommonAdvanced:  "Дополнительно",
		KeyCommonEnterPath: "Ввести путь…",

		KeyRailPlay:        "Играть",
		KeyRailInstall:     "Установка",
		KeyRailOpenMWCS:    "OpenMW-CS",
		KeyRailDiagnostics: "Диагностика",
		KeyRailAbout:       "О клиенте",

		KeyDiagnosticsTitle:           "Диагностика",
		KeyDiagnosticsIntro:           "Что клиент проверяет перед каждым запуском: пути, файлы игры, шрифты и настройки.",
		KeyDiagnosticsReport:          "Диагностический отчёт",
		KeyDiagnosticsReportHint:      "Пути, версии, настройки и последние ошибки запуска в одном текстовом файле.",
		KeyDiagnosticsReportPasswords: "Пароли серверов в отчёт не попадают.",
		KeyDiagnosticsReportLocal:     "Отчёт хранится только локально и никуда не отправляется самим клиентом.",
		KeyDiagnosticsCopyReport:      "Скопировать отчёт",
		KeyDiagnosticsReportCopied:    "Диагностический отчёт скопирован в буфер обмена.",
		KeyDiagnosticsFolders:         "Папки",
		KeyDiagnosticsFoldersHint:     "Пути открываются в системном файловом менеджере.",
		KeyDiagnosticsFolderWorkDir:   "Рабочая папка",
		KeyDiagnosticsFolderTES3MP:    "TES3MP",
		KeyDiagnosticsFolderOpenMW:    "Конфигурация OpenMW",
		KeyDiagnosticsFolderLog:       "Лог клиента",

		KeyInstallTitle:                "Установка",
		KeyInstallIntro:                "Здесь проверяется всё, что нужно клиенту. Пути выбираются автоматически, папка Morrowind только читается.",
		KeyInstallRunWizard:            "Запустить мастер",
		KeyInstallMorrowind:            "Morrowind",
		KeyInstallTES3MP:               "TES3MP",
		KeyInstallMorrowindMissing:     "Папка Morrowind ещё не выбрана.",
		KeyInstallChangeFolder:         "Сменить папку",
		KeyInstallReinstall:            "Переустановить",
		KeyInstallFonts:                "Шрифты",
		KeyInstallFontsReady:           "Набор шрифтов %s установлен.",
		KeyInstallFontsReadyDetail:     "Набор шрифтов %s установлен в %s.",
		KeyInstallFontsPartial:         "Набор шрифтов неполный, не хватает: %s",
		KeyInstallFontsMissing:         "Шрифты пока не установлены.",
		KeyInstallFontsMissingSet:      "Набор шрифтов %s пока не установлен.",
		KeyInstallFontsInstall:         "Установить шрифты и продолжить",
		KeyFontsSet:                    "Набор шрифтов",
		KeyFontsSetHint:                "Шрифт интерфейса, слотов Daedric и консоли. Каждый набор покрывает латиницу и кириллицу; загрузка проверяется по SHA256 и не блокирует игру.",
		KeyFontsSetLicense:             "Все наборы — шрифты Google Fonts под лицензией OFL, скачиваются из официального репозитория.",
		KeyFontsSetDefault:             "(по умолчанию)",
		KeyFontsSetChange:              "Сменить набор",
		KeyFontsSetApply:               "Применить и установить",
		KeyFontsSetRequired:            "Сначала выберите набор шрифтов.",
		KeyFontsDownloading:            "Загрузка набора шрифтов…",
		KeyFontsDownloadingAsset:       "Загрузка %s…",
		KeyFontsDownloadingBytes:       "Загрузка %s… %s из %s",
		KeyFontsInstalling:             "Установка %s…",
		KeyFontsVerifying:              "Проверка набора шрифтов…",
		KeyFontsCanceled:               "Загрузка шрифтов отменена.",
		KeyFontsDownloadFailed:         "Не удалось скачать шрифты.",
		KeyFontsCancel:                 "Отменить",
		KeyFontsRetry:                  "Повторить",
		KeyInstallLanguage:             "Язык игры",
		KeyInstallLanguageOriginal:     "Пакет не установлен — игра остаётся на исходном языке Morrowind.",
		KeyInstallLanguageText:         "%s · текст",
		KeyInstallLanguageVoices:       "%s · текст + озвучка",
		KeyInstallInstall:              "Установить",
		KeyInstallChange:               "Сменить…",
		KeyInstallBrowse:               "Выбрать…",
		KeyInstallWorkDirHint:          "Оставьте по умолчанию. Смена папки после установки требует заново установить TES3MP и моды.",
		KeyInstallProtonFound:          "Найдено автоматически",
		KeyInstallProtonManual:         "Задано вручную",
		KeyInstallProtonMissing:        "Не найден — выберите папку",
		KeyUninstall:                   "Удалить установку",
		KeyUninstallDangerZone:         "Опасная зона",
		KeyUninstallTitle:              "Удаление TES3MP Easy",
		KeyUninstallHint:               "Выбранные папки удаляются с диска без возможности восстановления. Установка Morrowind не изменяется.",
		KeyUninstallGameFiles:          "Файлы игры: TES3MP, серверы, моды, шрифты и локализация",
		KeyUninstallCache:              "Скачанные архивы (кэш)",
		KeyUninstallSettings:           "Настройки клиента и профили серверов",
		KeyUninstallKeepMorrowind:      "Morrowind, его Data Files и настройки самой игры остаются на месте.",
		KeyUninstallEmpty:              "Отметьте хотя бы один пункт.",
		KeyUninstallRunning:            "Удаление файлов…",
		KeyUninstallDone:               "Удалено:\n%s",
		KeyUninstallRestart:            "Настройки клиента удалены. Лаунчер сейчас закроется, следующий запуск начнётся с первичной настройки.",
		KeyUninstallRemove:             "Удалить",
		KeyInstallUIVideo:              "Интерфейс и видео",
		KeyInstallUIVideoConfigure:     "Настроить",
		KeyInstallUIVideoReady:         "Настроено: в settings.cfg записаны значения лаунчера.",
		KeyInstallUIVideoNoFile:        "settings.cfg появится после первого запуска; настроить можно уже сейчас.",
		KeyInstallUIVideoDefaults:      "Пока не настроено: используются значения OpenMW по умолчанию.",
		KeyInstallUIVideoTitle:         "Интерфейс и видео",
		KeyInstallUIVideoHint:          "Меняются только четыре значения ниже. Комментарии, остальные ключи и переводы строк в settings.cfg остаются как были; изменения применятся при следующем запуске.",
		KeyInstallUIVideoTTF:           "Разрешение TTF",
		KeyInstallUIVideoTTFHint:       "Размер атласа глифов: выше — чётче буквы при крупном шрифте и больше видеопамяти. Именно это убирает размытый стандартный текст.",
		KeyInstallUIVideoFontSize:      "Размер шрифта",
		KeyInstallUIVideoFontSizeHint:  "Базовый размер текста интерфейса. Крупный шрифт на маленьком атласе выглядит грубо — поднимайте оба значения вместе.",
		KeyInstallUIVideoScaling:       "Масштаб интерфейса",
		KeyInstallUIVideoScalingHint:   "Масштаб меню и HUD: 1.00 — исходный размер, 1.50–2.00 удобно на 1440p и 4K.",
		KeyInstallUIVideoFramerate:     "Ограничение FPS",
		KeyInstallUIVideoFramerateHint: "0 — без ограничения. 60 хватает Morrowind; на экране 120–144 Гц ставьте 120 или 144.",
		KeyInstallUIVideoInvalid:       "Введите значение в диапазоне, указанном рядом с подписью.",
		KeyInstallUIVideoUnreadable:    "settings.cfg не читается: %s",
		KeyInstallUIVideoSaved:         "settings.cfg обновлён.",

		KeyCSHeading:      "OpenMW-CS",
		KeyCSEditor:       "Редактор",
		KeyCSIntro:        "OpenMW Construction Set видит те же данные Morrowind и моды активного сервера, что и игра.",
		KeyCSFound:        "Редактор установлен: %s",
		KeyCSMissing:      "Редактор отсутствует",
		KeyCSExpected:     "Ожидается: %s",
		KeyCSLaunch:       "Запустить редактор",
		KeyCSCheckAgain:   "Проверить снова",
		KeyCSOpenFolder:   "Открыть папку",
		KeyCSModsRequired: "Обновите моды активного сервера, прежде чем открывать редактор.",
		KeyCSNoServer:     "Сначала добавьте сервер: редактор открывает его моды.",
		KeyCSProtonNote:   "На Linux редактор запускается в том же префиксе Proton, что и игра, иначе он не увидит Documents/My Games/OpenMW.",
		KeyCSStarted:      "OpenMW-CS запущен.",

		KeyServerListTitle:   "Серверы",
		KeyServerListHint:    "Одна карточка на сервер. Play скачивает недостающие моды и записывает endpoint.",
		KeyServerEmptyTitle:  "Серверов пока нет",
		KeyServerEmptyHint:   "Добавьте адрес сервера, который прислали друзья.",
		KeyServerActive:      "активен",
		KeyServerPanel:       "Панель",
		KeyServerAddTitle:    "Добавить сервер",
		KeyServerEditTitle:   "Изменить сервер",
		KeyServerDeleteTitle: "Удалить сервер",
		KeyServerDeleteBody:  "Удалить профиль %q? Уже скачанные файлы останутся на диске.",
		KeyServerFieldName:   "Название",
		KeyServerFieldAddr:   "Адрес",
		KeyServerFieldPass:   "Пароль",
		KeyServerFieldPanel:  "URL панели",
		KeyServerUpdateMods:  "Обновить моды",
		KeyServerLaunching:   "Подготовка к запуску...",
		KeyServerLaunched:    "Игра запущена.",

		KeyServerAlreadyStarting: "TES3MP уже запускается. Дождитесь появления окна игры.",
		KeyServerAlreadyRunning:  "TES3MP уже запущен. Закройте игру, чтобы запустить её снова.",

		KeyServerMorrowindRequired: "Укажите папку Morrowind на экране «Установка», чтобы играть.",

		KeyServerMore:            "Ещё",
		KeyServerActionsTitle:    "Действия с сервером",
		KeyServerOpenPanel:       "Открыть панель",
		KeyServerPlayDownload:    "Скачать моды и играть",
		KeyServerPlayUpdate:      "Обновить и играть",
		KeyServerStatusOnline:    "Онлайн · %s",
		KeyServerStatusUpdate:    "доступно обновление модов",
		KeyServerStatusChecking:  "Проверка сервера...",
		KeyServerPreparing:       "Подготовка к запуску...",
		KeyServerEndpointHint:    "Вставьте хост, host:port или адрес панели https://.",
		KeyServerAdvancedOptions: "Дополнительные настройки",
		KeyServerPanelHint:       "Оставьте пустым, чтобы использовать https://<адрес> для публичной панели.",

		KeyModsTitle:       "Моды сервера",
		KeyModsChecking:    "Проверка модов на сервере...",
		KeyModsDownloading: "Загрузка модов: %s из %s",
		KeyModsVerifying:   "Проверка архива модов...",
		KeyModsExtracting:  "Распаковка модов...",
		KeyModsInstalling:  "Установка модов...",
		KeyModsUpToDate:    "Моды уже актуальны.",
		KeyModsDone:        "Моды установлены.",

		KeySettingsTheme:   "Тема",
		KeySettingsWorkDir: "Рабочая папка",
		KeySettingsProton:  "Proton (Linux)",

		KeyThemeSystem: "Как в системе",
		KeyThemeDark:   "Тёмная",
		KeyThemeLight:  "Светлая",

		KeyLocaleTitle:              "Язык игры",
		KeyLocaleQuestion:           "Языковой пакет заменяет текст игры и, при желании, озвучку. Без пакета игра идёт на исходном языке — это нормальная настройка, а не ошибка.",
		KeyLocaleOnlyPack:           "Единственный пакет в каталоге",
		KeyLocaleVariantText:        "Только текст",
		KeyLocaleVariantTextHint:    "≈%s · текст интерфейса и субтитры диалогов · кодировка %s",
		KeyLocaleVariantVoices:      "Текст и озвучка",
		KeyLocaleVariantVoicesHint:  "≈%s · добавляет озвученные диалоги · загрузка идёт дольше",
		KeyLocaleNoVoices:           "У этого пакета нет озвучки; устанавливается только текст.",
		KeyLocaleInstalled:          "Установлено: %s",
		KeyLocaleInstall:            "Установить языковой пакет…",
		KeyLocaleChange:             "Изменить…",
		KeyLocaleChecking:           "Проверка установленных языковых файлов...",
		KeyLocaleDownloading:        "Загрузка языковых файлов: %s из %s",
		KeyLocaleExtracting:         "Распаковка языковых файлов...",
		KeyLocaleInstalling:         "Применение языковых файлов...",
		KeyLocaleRemoved:            "Языковой overlay удалён.",
		KeyLanguageWizardRemoveHint: "Языковой overlay будет удалён; все профили вернутся к оригинальному языку Morrowind.",
		KeyLanguageWizardRemove:     "Удалить",
		KeyLanguageWizardApplying:   "Установка локализации...",
		KeyLanguageWizardDone:       "Локализация установлена для всех серверов.",

		KeyErrorWhat:         "Что случилось: %s",
		KeyErrorDetail:       "Технические подробности: %s",
		KeyErrorActionLine:   "Что сделать: %s",
		KeyErrorLogLine:      "Лог: %s",
		KeyErrorRetry:        "Повторить",
		KeyErrorUnknown:      "Операция не удалась.",
		KeyErrorUnknownAct:   "Откройте лог, проверьте подробности и попробуйте ещё раз.",
		KeyErrorCode:         "Код поддержки: %s",
		KeyErrorShowDetails:  "Технические подробности",
		KeyErrorNoDetails:    "Технических подробностей нет.",
		KeyErrorOpenLog:      "Открыть лог",
		KeyErrorCopyReport:   "Скопировать отчёт",
		KeyErrorReportCopied: "Диагностический отчёт скопирован без пароля сервера.",

		KeyErrorMorrowindNotFound:      "Morrowind не найден.",
		KeyErrorMorrowindNotFoundAct:   "Выберите папку, в которой есть Data Files/Morrowind.esm, или сначала установите Morrowind.",
		KeyErrorMorrowindIncomplete:    "Папка Morrowind неполная.",
		KeyErrorMorrowindIncompleteAct: "Выберите папку с Data Files/Morrowind.esm, Tribunal.esm и Bloodmoon.esm.",
		KeyErrorMorrowindMissing:       "Папка Morrowind не выбрана.",
		KeyErrorMorrowindMissingAct:    "Выберите папку Morrowind в мастере настройки или в Настройках.",
		KeyErrorNetwork:                "Сервер не отвечает.",
		KeyErrorNetworkAct:             "Проверьте адрес панели, интернет, VPN и файрвол, затем повторите.",
		KeyErrorTimeout:                "Сервер слишком долго не отвечает.",
		KeyErrorTimeoutAct:             "Повторите через минуту. Если таймаут повторяется, проверьте сервер и соединение.",
		KeyErrorCertificate:            "Не удалось проверить сертификат панели.",
		KeyErrorCertificateAct:         "Проверьте системную дату и адрес панели. Не отключайте проверку сертификатов.",
		KeyErrorNoMods:                 "На панели нет актуального набора модов.",
		KeyErrorNoModsAct:              "Попросите владельца сервера опубликовать моды в панели и повторите.",
		KeyErrorServerAPI:              "Панель вернула ошибку.",
		KeyErrorServerAPIAct:           "Проверьте адрес панели и повторите. Если ошибка повторяется, отправьте лог владельцу сервера.",
		KeyErrorFileSystem:             "Не удалось прочитать или записать файл.",
		KeyErrorFileSystemAct:          "Проверьте свободное место и права на запись в рабочей папке, затем повторите.",
		KeyErrorWorkDir:                "Рабочая папка некорректна.",
		KeyErrorWorkDirAct:             "Выберите существующую папку с абсолютным путём, доступную для записи, в мастере или Настройках.",
		KeyErrorModsMissing:            "Моды сервера не установлены.",
		KeyErrorModsMissingAct:         "Запустите мастер настройки или нажмите «Обновить моды», затем повторите.",
		KeyErrorLocaleMissing:          "Установленная локализация отсутствует или повреждена.",
		KeyErrorLocaleMissingAct:       "Откройте «Установку» и установите языковой пакет заново.",
		KeyErrorGameMissing:            "TES3MP не установлен.",
		KeyErrorGameMissingAct:         "Откройте мастер подготовки, установите TES3MP и повторите.",
		KeyErrorGameProton:             "Proton не удаётся использовать.",
		KeyErrorGameProtonAct:          "Выберите рабочую папку Proton в «Установка → Дополнительно» или установите Proton через Steam.",
		KeyErrorGameConfig:             "Не удалось подготовить конфигурацию игры.",
		KeyErrorGameConfigAct:          "Проверьте права на запись в рабочей папке и папке конфигурации OpenMW, затем повторите.",
		KeyErrorGameStart:              "Не удалось запустить TES3MP.",
		KeyErrorGameStartAct:           "Проверьте установку TES3MP, закройте другие копии игры и повторите.",
		KeyErrorGameExited:             "TES3MP неожиданно завершился.",
		KeyErrorGameExitedAct:          "Откройте лог клиента, найдите ошибку игры и повторите после исправления.",
		KeyErrorGameStarting:           "TES3MP ещё запускается.",
		KeyErrorGameStartingAct:        "Дождитесь появления окна игры, а не запускайте её повторно.",
		KeyErrorGameRunning:            "TES3MP уже запущен.",
		KeyErrorGameRunningAct:         "Закройте окно игры и повторите запуск, чтобы создать новый экземпляр.",

		KeyLogsTitle:      "Лог клиента",
		KeyLogsEmpty:      "Лог пока пуст.",
		KeyLogsRefresh:    "Обновить",
		KeyLogsOpenFolder: "Открыть папку с логом",
		KeyLogsHint:       "Показаны последние %d строк. Полный файл лежит в папке с логом.",

		KeyWizardTitle:          "Первая настройка",
		KeyWizardStep:           "Шаг %d из %d",
		KeyWizardCancel:         "Отменить настройку",
		KeyWizardBack:           "Назад",
		KeyWizardNext:           "Далее",
		KeyWizardFinish:         "Готово",
		KeyWizardRetry:          "Повторить",
		KeyWizardMorrowindTitle: "Morrowind",
		KeyWizardMorrowindHint:  "Выберите папку с оригинальной игрой. Клиент только читает её.",
		KeyWizardFontsTitle:     "Шрифты и интерфейс",
		KeyWizardFontsHint:      "Файлы заменяют низкоприоритетные стандартные шрифты в My Games/OpenMW/Fonts, а в openmw.cfg добавляются строки fallback= — текст перестаёт быть размытым на высоком разрешении, а кириллица — квадратиками. Набор проверяется перед каждым запуском; неудачная загрузка повторяется при следующем Play. Четыре значения ниже — те же, что диалог «Настроить» пишет в settings.cfg; диапазон указан в подписи.",
		KeyWizardFontsPending:   "Набор шрифтов %s будет скачан в фоне после настройки; Play из-за этого не блокируется.",
		KeyWizardFontsPartial:   "Часть шрифтов отсутствует (%s). Они будут скачаны заново в фоне.",
		KeyFontsPreviewHint:     "Образец набран установленным файлом выбранного набора; до загрузки используется шрифт лаунчера.",
		KeyWizardTES3MPTitle:    "TES3MP",
		KeyWizardTES3MPHint:     "Клиент скачает и проверит TES3MP %s. Адрес сервера для этого шага не нужен.",

		KeySetupFind:                "Найти автоматически",
		KeySetupChoose:              "Выбрать папку",
		KeySetupCheck:               "Проверить файлы",
		KeySetupSearching:           "Ищу Morrowind...",
		KeySetupChecking:            "Проверяю папку...",
		KeySetupMorrowindReady:      "Morrowind готов: %s",
		KeySetupMorrowindIncomplete: "Нельзя использовать эту папку: %s",
		KeySetupMorrowindNotFound:   "Morrowind не найден. Выберите папку с игрой или продолжите без неё.",
		KeySetupTES3MP:              "TES3MP %s",
		KeySetupTES3MPReady:         "TES3MP %s установлен.",
		KeySetupTES3MPOutdated:      "Установлен TES3MP %s, нужен %s.",
		KeySetupTES3MPMissing:       "TES3MP ещё не установлен.",
		KeySetupInstall:             "Установить TES3MP",
		KeySetupDownloading:         "Загрузка TES3MP: %s из %s",
		KeySetupStarting:            "Подготовка загрузки TES3MP...",
		KeySetupVerifying:           "Проверка архива TES3MP...",
		KeySetupExtracting:          "Распаковка TES3MP...",
		KeySetupInstalling:          "Завершение установки TES3MP...",
		KeySetupDone:                "TES3MP установлен и готов.",

		KeyAboutTitle:        "О клиенте",
		KeyAboutIntro:        "Клиент, который устанавливает и запускает TES3MP с нужными шрифтами и настройками — без ручного редактирования файлов.",
		KeyAboutVersion:      "Версия %s",
		KeyAboutHint:         "Из чего собран клиент и что установлено.",
		KeyAboutLicense:      "Лицензия",
		KeyAboutLicenseValue: "MIT",
		KeyAboutOpenSource:   "Открытый код",
		KeyAboutProject:      "Проект",
		KeyAboutOpenPage:     "Открыть страницу проекта",
		KeyAboutComponents:   "Компоненты",
		KeyAboutTES3MP:       "TES3MP %s",
		KeyAboutTES3MPHint:   "Сетевая версия OpenMW для игры на серверах.",
		KeyAboutOpenMW:       "OpenMW",
		KeyAboutOpenMWHint:   "Свободный движок, который клиент настраивает и запускает.",
		KeyAboutFyne:         "Fyne",
		KeyAboutFyneHint:     "Инструментарий, которым нарисовано это окно.",
		KeyAboutNotInstalled: "Не установлен",
		KeyAboutReady:        "Установлен",
		KeyAboutSystem:       "Система",
		KeyAboutLegal:        "Morrowind и его файлы принадлежат Bethesda Softworks. Клиент не изменяет папку игры.",
		KeyFooterPath:        "Конфиг: %s",
		KeyFooterVersion:     "tes3mp-easy %s · TES3MP %s",

		KeyLanguageRussian: "Русский",
	},
}
