package i18n

// Stable translation keys used by the UI.
const (
	KeyAppTitle   = "app.title"
	KeyAppTagline = "app.tagline"

	KeyCommonAdd      = "common.add"
	KeyCommonEdit     = "common.edit"
	KeyCommonDelete   = "common.delete"
	KeyCommonSave     = "common.save"
	KeyCommonCancel   = "common.cancel"
	KeyCommonClose    = "common.close"
	KeyCommonMenu     = "common.menu"
	KeyCommonAbout    = "common.about"
	KeyCommonPlay     = "common.play"
	KeyCommonLanguage = "common.language"
	KeyCommonError    = "common.error"
	KeyCommonSetup    = "common.setup"
	KeyCommonAdvanced = "common.advanced"

	KeyRailPlay         = "rail.play"
	KeyRailInstall      = "rail.installation"
	KeyRailOpenMWCS     = "rail.openmw_cs"
	KeyRailInstallBadge = "rail.installation.badge"
	KeyRailMenuNote     = "rail.menu_note"

	KeyMenuCopyReport   = "menu.copy_report"
	KeyMenuReportCopied = "menu.report_copied"

	KeyInstallTitle               = "install.title"
	KeyInstallIntro               = "install.intro"
	KeyInstallRunWizard           = "install.run_wizard"
	KeyInstallMorrowind           = "install.morrowind"
	KeyInstallMorrowindMissing    = "install.morrowind.missing"
	KeyInstallCheck               = "install.check"
	KeyInstallChangeFolder        = "install.change_folder"
	KeyInstallOpenFolder          = "install.open_folder"
	KeyInstallReinstall           = "install.reinstall"
	KeyInstallFonts               = "install.fonts"
	KeyInstallFontsReady          = "install.fonts.ready"
	KeyInstallFontsReadyDetail    = "install.fonts.ready_detail"
	KeyInstallFontsPartial        = "install.fonts.partial"
	KeyInstallFontsMissing        = "install.fonts.missing"
	KeyInstallFontsSkipped        = "install.fonts.skipped"
	KeyInstallFontsMissingSet     = "install.fonts.missing_set"
	KeyFontsSet                   = "fonts.set"
	KeyFontsSetHint               = "fonts.set.hint"
	KeyFontsSetLicense            = "fonts.set.license"
	KeyFontsSetNoto               = "fonts.set.noto"
	KeyFontsSetInter              = "fonts.set.inter"
	KeyFontsSetPlex               = "fonts.set.plex"
	KeyFontsSetChange             = "fonts.set.change"
	KeyFontsSetApply              = "fonts.set.apply"
	KeyFontsSetRequired           = "fonts.set.required"
	KeyInstallLocalization        = "install.localization"
	KeyInstallLocalizationMissing = "install.localization.missing"
	KeyInstallLocalizationVoice   = "install.localization.voice"
	KeyInstallLocalizationVoices  = "install.localization.voices"
	KeyInstallLocalizationHint    = "install.localization.voices_hint"
	KeyInstallInstall             = "install.install"
	KeyInstallChange              = "install.change"
	KeyInstallWorkDirHint         = "install.work_dir_hint"
	KeyInstallProtonHint          = "install.proton_hint"
	KeyInstallFolders             = "install.folders"
	KeyInstallFolderTES3MP        = "install.folders.tes3mp"
	KeyInstallFolderConfig        = "install.folders.openmw_config"
	KeyInstallDiagnostics         = "install.diagnostics"
	KeyInstallProfileAuto         = "install.profile_auto"

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
	KeyInstallUIVideoFontSize      = "install.uivideo.font_size"
	KeyInstallUIVideoScaling       = "install.uivideo.scaling"
	KeyInstallUIVideoFramerate     = "install.uivideo.framerate"
	KeyInstallUIVideoFramerateHint = "install.uivideo.framerate_hint"
	KeyInstallUIVideoRanges        = "install.uivideo.ranges"
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

	KeyLocaleInstallTitle                = "locale.install.title"
	KeyLocaleChecking                    = "locale.checking"
	KeyLocaleDownloading                 = "locale.downloading"
	KeyLocaleExtracting                  = "locale.extracting"
	KeyLocaleInstalling                  = "locale.installing"
	KeyLocaleRemoved                     = "locale.removed"
	KeyLanguageWizardChooseTitle         = "language.choose.title"
	KeyLanguageWizardChooseHint          = "language.choose.hint"
	KeyLanguageWizardRussianHint         = "language.russian.hint"
	KeyLanguageWizardRemoveHint          = "language.remove.hint"
	KeyLanguageWizardInstallTitle        = "language.install.title"
	KeyLanguageWizardInstallHint         = "language.install.hint"
	KeyLanguageWizardCurrent             = "language.current"
	KeyLanguageWizardStateMissing        = "language.state.missing"
	KeyLanguageWizardStateInstalled      = "language.state.installed"
	KeyLanguageWizardStateInstalledVoice = "language.state.installed_voice"
	KeyLanguageWizardRemove              = "language.remove"
	KeyLanguageWizardApplying            = "language.applying"
	KeyLanguageWizardDone                = "language.done"

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

	KeyLogsTitle      = "logs.title"
	KeyLogsEmpty      = "logs.empty"
	KeyLogsRefresh    = "logs.refresh"
	KeyLogsOpenFolder = "logs.open_folder"
	KeyLogsHint       = "logs.hint"
	KeyLogsExitHint   = "logs.exit_hint"

	KeyWizardTitle             = "wizard.title"
	KeyWizardStep              = "wizard.step"
	KeyWizardCancel            = "wizard.cancel"
	KeyWizardBack              = "wizard.back"
	KeyWizardNext              = "wizard.next"
	KeyWizardFinish            = "wizard.finish"
	KeyWizardRetry             = "wizard.retry"
	KeyWizardMorrowindTitle    = "wizard.morrowind.title"
	KeyWizardMorrowindHint     = "wizard.morrowind.hint"
	KeyWizardFontsTitle        = "wizard.fonts.title"
	KeyWizardFontsHint         = "wizard.fonts.hint"
	KeyWizardFontsDownloading  = "wizard.fonts.downloading"
	KeyWizardFontsFailed       = "wizard.fonts.failed"
	KeyWizardUIVideoTitle      = "wizard.uivideo.title"
	KeyWizardUIVideoHint       = "wizard.uivideo.hint"
	KeyWizardLocalizationTitle = "wizard.localization.title"
	KeyWizardLocalizationHint  = "wizard.localization.hint"
	KeyWizardTES3MPTitle       = "wizard.tes3mp.title"
	KeyWizardTES3MPHint        = "wizard.tes3mp.hint"
	KeyWizardReadyTitle        = "wizard.ready.title"
	KeyWizardReadyHint         = "wizard.ready.hint"
	KeyWizardSummary           = "wizard.ready.summary"

	KeySetupFind                = "setup.find"
	KeySetupChoose              = "setup.choose"
	KeySetupCheck               = "setup.check"
	KeySetupMorrowindReady      = "setup.morrowind.ready"
	KeySetupMorrowindIncomplete = "setup.morrowind.incomplete"
	KeySetupTES3MP              = "setup.tes3mp"
	KeySetupTES3MPReady         = "setup.tes3mp.ready"
	KeySetupTES3MPOutdated      = "setup.tes3mp.outdated"
	KeySetupTES3MPMissing       = "setup.tes3mp.missing"
	KeySetupInstall             = "setup.install"
	KeySetupDownloading         = "setup.downloading"
	KeySetupVerifying           = "setup.verifying"
	KeySetupExtracting          = "setup.extracting"
	KeySetupInstalling          = "setup.installing"
	KeySetupDone                = "setup.done"
	KeySetupLocalizationInstall = "setup.localization.install"
	KeySetupLocalizationReady   = "setup.localization.ready"
	KeySetupLocalizationMissing = "setup.localization.missing"

	KeyAboutTitle = "about.title"
	KeyAboutBody  = "about.body"
	KeyFooterPath = "footer.config_path"

	KeyLanguageRussian = "lang.russian"
)

var catalogs = map[Language]map[string]string{
	English: {
		KeyAppTitle:   "tes3mp-easy",
		KeyAppTagline: "Morrowind with friends",

		KeyCommonAdd:      "Add",
		KeyCommonEdit:     "Edit",
		KeyCommonDelete:   "Delete",
		KeyCommonSave:     "Save",
		KeyCommonCancel:   "Cancel",
		KeyCommonClose:    "Close",
		KeyCommonMenu:     "Menu",
		KeyCommonAbout:    "About",
		KeyCommonPlay:     "Play",
		KeyCommonLanguage: "Language",
		KeyCommonError:    "Error",
		KeyCommonSetup:    "Setup",
		KeyCommonAdvanced: "Advanced",

		KeyRailPlay:         "Play",
		KeyRailInstall:      "Installation",
		KeyRailOpenMWCS:     "OpenMW-CS",
		KeyRailInstallBadge: "%s · %d",
		KeyRailMenuNote:     "Language, theme, client log, report and About are in the ⋯ menu.",

		KeyMenuCopyReport:   "Copy report",
		KeyMenuReportCopied: "The diagnostic report was copied to the clipboard.",

		KeyInstallTitle:                "Installation",
		KeyInstallIntro:                "Everything the client needs is checked here. Paths are automatic and the Morrowind folder is only read.",
		KeyInstallRunWizard:            "Run wizard",
		KeyInstallMorrowind:            "Morrowind",
		KeyInstallMorrowindMissing:     "Morrowind folder is not selected yet.",
		KeyInstallCheck:                "Check",
		KeyInstallChangeFolder:         "Change folder",
		KeyInstallOpenFolder:           "Open folder",
		KeyInstallReinstall:            "Reinstall",
		KeyInstallFonts:                "Fonts",
		KeyInstallFontsReady:           "The %s font set is installed.",
		KeyInstallFontsReadyDetail:     "The %s font set is installed in %s.",
		KeyInstallFontsPartial:         "The font set is incomplete, missing: %s",
		KeyInstallFontsMissing:         "Fonts are not installed yet.",
		KeyInstallFontsSkipped:         "Fonts are skipped for launch (legacy option).",
		KeyInstallFontsMissingSet:      "The %s font set is not installed yet.",
		KeyFontsSet:                    "Font set",
		KeyFontsSetHint:                "The interface, Daedric slots and console font. Every set covers Latin and Cyrillic; the download is verified and does not block the game.",
		KeyFontsSetLicense:             "All sets are OFL-licensed Google Fonts and are downloaded from the official repository.",
		KeyFontsSetNoto:                "Noto Sans · Noto Serif · Roboto Mono (default)",
		KeyFontsSetInter:               "Inter · Source Serif · JetBrains Mono",
		KeyFontsSetPlex:                "IBM Plex Sans · Plex Serif · Plex Mono",
		KeyFontsSetChange:              "Change set",
		KeyFontsSetApply:               "Apply and install",
		KeyFontsSetRequired:            "Choose a font set first.",
		KeyInstallLocalization:         "Localization",
		KeyInstallLocalizationMissing:  "Not installed — the original Morrowind language is used.",
		KeyInstallLocalizationVoice:    "%s + voiceover",
		KeyInstallLocalizationVoices:   "Install Russian voiceover",
		KeyInstallLocalizationHint:     "Adds the Russian voice files to the localization. A large download; Russian text works without it.",
		KeyInstallInstall:              "Install",
		KeyInstallChange:               "Change…",
		KeyInstallWorkDirHint:          "Keep the default. Changing the folder after installation requires installing TES3MP and the mods again.",
		KeyInstallProtonHint:           "Only needed on Linux when Steam does not provide Proton.",
		KeyInstallFolders:              "Folders",
		KeyInstallFolderTES3MP:         "TES3MP",
		KeyInstallFolderConfig:         "OpenMW config",
		KeyInstallDiagnostics:          "Diagnostics",
		KeyInstallProfileAuto:          "The client profile is automatic: openmw.cfg and the endpoint are rebuilt from the active server on every Play.",
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
		KeyInstallUIVideoFontSize:      "Font size",
		KeyInstallUIVideoScaling:       "Scaling factor",
		KeyInstallUIVideoFramerate:     "Framerate limit",
		KeyInstallUIVideoFramerateHint: "0 means no limit",
		KeyInstallUIVideoRanges:        "TTF resolution 16–48, font size 8–32, scaling 0.5–4, framerate 0–360.",
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

		KeyThemeSystem: "Follow system",
		KeyThemeDark:   "Amber dark",
		KeyThemeLight:  "Amber light",

		KeyLocaleInstallTitle:                "Install localization",
		KeyLocaleChecking:                    "Checking the installed language files...",
		KeyLocaleDownloading:                 "Downloading language files: %s of %s",
		KeyLocaleExtracting:                  "Extracting language files...",
		KeyLocaleInstalling:                  "Applying language files...",
		KeyLocaleRemoved:                     "The localization overlay was removed.",
		KeyLanguageWizardChooseTitle:         "Localization",
		KeyLanguageWizardChooseHint:          "Choose a localization package. It is installed once for all server profiles; the original Morrowind folder is never modified.",
		KeyLanguageWizardRussianHint:         "Downloads Russian text; the optional voiceover adds the Russian voice files. Cyrillic fonts are installed automatically. Large download.",
		KeyLanguageWizardRemoveHint:          "The localization overlay will be removed; every profile returns to the original Morrowind language.",
		KeyLanguageWizardInstallTitle:        "Installing localization",
		KeyLanguageWizardInstallHint:         "Do not close the client until the installation finishes.",
		KeyLanguageWizardCurrent:             "Current: %s",
		KeyLanguageWizardStateMissing:        "not installed",
		KeyLanguageWizardStateInstalled:      "%s, installed",
		KeyLanguageWizardStateInstalledVoice: "%s, installed with voiceover",
		KeyLanguageWizardRemove:              "Remove localization",
		KeyLanguageWizardApplying:            "Installing localization...",
		KeyLanguageWizardDone:                "Localization is installed for all server profiles.",

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
		KeyErrorLocaleMissingAct:       "Open Menu → Localization and install the package again.",
		KeyErrorGameMissing:            "TES3MP is not installed.",
		KeyErrorGameMissingAct:         "Open Setup and install TES3MP, then retry.",
		KeyErrorGameProton:             "The Proton runtime is not usable.",
		KeyErrorGameProtonAct:          "Select a working Proton folder in Settings, or install Proton through Steam.",
		KeyErrorGameConfig:             "The game configuration could not be prepared.",
		KeyErrorGameConfigAct:          "Check write permissions for the work folder and the OpenMW config folder, then retry.",
		KeyErrorGameStart:              "TES3MP could not be started.",
		KeyErrorGameStartAct:           "Check that TES3MP is installed correctly, close other instances and retry.",
		KeyErrorGameExited:             "TES3MP exited unexpectedly.",
		KeyErrorGameExitedAct:          "Open the client log for the game's error and retry after fixing it.",

		KeyLogsTitle:      "Client log",
		KeyLogsEmpty:      "The log is empty yet.",
		KeyLogsRefresh:    "Refresh",
		KeyLogsOpenFolder: "Open log folder",
		KeyLogsHint:       "The last %d lines are shown. The full file is in the log folder.",
		KeyLogsExitHint:   "If the game closed unexpectedly, open the client log.",

		KeyWizardTitle:             "First-run setup",
		KeyWizardStep:              "Step %d of %d",
		KeyWizardCancel:            "Cancel setup",
		KeyWizardBack:              "Back",
		KeyWizardNext:              "Next",
		KeyWizardFinish:            "Finish",
		KeyWizardRetry:             "Retry",
		KeyWizardMorrowindTitle:    "Morrowind",
		KeyWizardMorrowindHint:     "Select the original game folder. The client only reads it.",
		KeyWizardFontsTitle:        "Fonts",
		KeyWizardFontsHint:         "The game shows boxes instead of Cyrillic without this set. It is downloaded once and re-checked before every launch; a failed download does not block the game.",
		KeyWizardFontsDownloading:  "Downloading the font set...",
		KeyWizardFontsFailed:       "Some fonts could not be downloaded: %s. The game still starts with the default font, and the next Play retries.",
		KeyWizardUIVideoTitle:      "UI and video",
		KeyWizardUIVideoHint:       "Optional step: set the text size and framerate now, or leave the values as they are.",
		KeyWizardLocalizationTitle: "Localization (optional)",
		KeyWizardLocalizationHint:  "Install a language pack once for all servers. You can skip this step and install it later from the Installation screen.",
		KeyWizardTES3MPTitle:       "TES3MP",
		KeyWizardTES3MPHint:        "The client downloads and verifies TES3MP %s. No server details are needed for this step.",
		KeyWizardReadyTitle:        "Ready to play",
		KeyWizardReadyHint:         "Setup is complete. Add a server from the main window when you are ready to play.",
		KeyWizardSummary:           "Morrowind: %s\nWork folder: %s\nTES3MP: %s\nLog: %s",

		KeySetupFind:                "Find automatically",
		KeySetupChoose:              "Choose folder",
		KeySetupCheck:               "Check files",
		KeySetupMorrowindReady:      "Morrowind is ready: %s",
		KeySetupMorrowindIncomplete: "Cannot use this folder: %s",
		KeySetupTES3MP:              "TES3MP %s",
		KeySetupTES3MPReady:         "TES3MP %s is installed.",
		KeySetupTES3MPOutdated:      "TES3MP %s is installed, but %s is required.",
		KeySetupTES3MPMissing:       "TES3MP is not installed yet.",
		KeySetupInstall:             "Install TES3MP",
		KeySetupDownloading:         "Downloading TES3MP: %s of %s",
		KeySetupVerifying:           "Checking the TES3MP archive...",
		KeySetupExtracting:          "Extracting TES3MP...",
		KeySetupInstalling:          "Finishing the TES3MP installation...",
		KeySetupDone:                "TES3MP is installed and ready.",
		KeySetupLocalizationInstall: "Install localization",
		KeySetupLocalizationReady:   "Localization is installed: %s",
		KeySetupLocalizationMissing: "Localization is not installed. You can skip this step.",

		KeyAboutTitle: "About",
		KeyAboutBody:  "tes3mp-easy client %s\nTES3MP %s\n%s",
		KeyFooterPath: "Config: %s",

		KeyLanguageRussian: "Russian",
	},
	Russian: {
		KeyAppTitle:   "tes3mp-easy",
		KeyAppTagline: "Morrowind с друзьями",

		KeyCommonAdd:      "Добавить",
		KeyCommonEdit:     "Изменить",
		KeyCommonDelete:   "Удалить",
		KeyCommonSave:     "Сохранить",
		KeyCommonCancel:   "Отмена",
		KeyCommonClose:    "Закрыть",
		KeyCommonMenu:     "Меню",
		KeyCommonAbout:    "О клиенте",
		KeyCommonPlay:     "Играть",
		KeyCommonLanguage: "Язык",
		KeyCommonError:    "Ошибка",
		KeyCommonSetup:    "Настройка",
		KeyCommonAdvanced: "Дополнительно",

		KeyRailPlay:         "Играть",
		KeyRailInstall:      "Установка",
		KeyRailOpenMWCS:     "OpenMW-CS",
		KeyRailInstallBadge: "%s · %d",
		KeyRailMenuNote:     "Язык, тема, лог клиента, отчёт и About — в меню «⋯».",

		KeyMenuCopyReport:   "Скопировать отчёт",
		KeyMenuReportCopied: "Диагностический отчёт скопирован в буфер обмена.",

		KeyInstallTitle:                "Установка",
		KeyInstallIntro:                "Здесь проверяется всё, что нужно клиенту. Пути выбираются автоматически, папка Morrowind только читается.",
		KeyInstallRunWizard:            "Запустить мастер",
		KeyInstallMorrowind:            "Morrowind",
		KeyInstallMorrowindMissing:     "Папка Morrowind ещё не выбрана.",
		KeyInstallCheck:                "Проверить",
		KeyInstallChangeFolder:         "Сменить папку",
		KeyInstallOpenFolder:           "Открыть папку",
		KeyInstallReinstall:            "Переустановить",
		KeyInstallFonts:                "Шрифты",
		KeyInstallFontsReady:           "Набор шрифтов %s установлен.",
		KeyInstallFontsReadyDetail:     "Набор шрифтов %s установлен в %s.",
		KeyInstallFontsPartial:         "Набор шрифтов неполный, не хватает: %s",
		KeyInstallFontsMissing:         "Шрифты пока не установлены.",
		KeyInstallFontsSkipped:         "Шрифты пропускаются при запуске (старая настройка).",
		KeyInstallFontsMissingSet:      "Набор шрифтов %s пока не установлен.",
		KeyFontsSet:                    "Набор шрифтов",
		KeyFontsSetHint:                "Шрифт интерфейса, слотов Daedric и консоли. Каждый набор покрывает латиницу и кириллицу; загрузка проверяется по SHA256 и не блокирует игру.",
		KeyFontsSetLicense:             "Все наборы — шрифты Google Fonts под лицензией OFL, скачиваются из официального репозитория.",
		KeyFontsSetNoto:                "Noto Sans · Noto Serif · Roboto Mono (по умолчанию)",
		KeyFontsSetInter:               "Inter · Source Serif · JetBrains Mono",
		KeyFontsSetPlex:                "IBM Plex Sans · Plex Serif · Plex Mono",
		KeyFontsSetChange:              "Сменить набор",
		KeyFontsSetApply:               "Применить и установить",
		KeyFontsSetRequired:            "Сначала выберите набор шрифтов.",
		KeyInstallLocalization:         "Локализация",
		KeyInstallLocalizationMissing:  "Не установлена — используется исходный язык Morrowind.",
		KeyInstallLocalizationVoice:    "%s + озвучка",
		KeyInstallLocalizationVoices:   "Установить русскую озвучку",
		KeyInstallLocalizationHint:     "Добавляет русские звуковые файлы к локализации. Большая загрузка; без них работает русский текст.",
		KeyInstallInstall:              "Установить",
		KeyInstallChange:               "Сменить…",
		KeyInstallWorkDirHint:          "Оставьте по умолчанию. Смена папки после установки требует заново установить TES3MP и моды.",
		KeyInstallProtonHint:           "Нужно только на Linux, если Proton не предоставлен Steam.",
		KeyInstallFolders:              "Папки",
		KeyInstallFolderTES3MP:         "TES3MP",
		KeyInstallFolderConfig:         "Конфигурация OpenMW",
		KeyInstallDiagnostics:          "Диагностика",
		KeyInstallProfileAuto:          "Профиль клиента автоматический: openmw.cfg и endpoint пересобираются из активного сервера при каждом запуске.",
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
		KeyInstallUIVideoFontSize:      "Размер шрифта",
		KeyInstallUIVideoScaling:       "Масштаб интерфейса",
		KeyInstallUIVideoFramerate:     "Ограничение FPS",
		KeyInstallUIVideoFramerateHint: "0 — без ограничения",
		KeyInstallUIVideoRanges:        "Разрешение TTF 16–48, размер шрифта 8–32, масштаб 0.5–4, FPS 0–360.",
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
		KeyThemeDark:   "Янтарная тёмная",
		KeyThemeLight:  "Янтарная светлая",

		KeyLocaleInstallTitle:                "Установить локализацию",
		KeyLocaleChecking:                    "Проверка установленных языковых файлов...",
		KeyLocaleDownloading:                 "Загрузка языковых файлов: %s из %s",
		KeyLocaleExtracting:                  "Распаковка языковых файлов...",
		KeyLocaleInstalling:                  "Применение языковых файлов...",
		KeyLocaleRemoved:                     "Языковой overlay удалён.",
		KeyLanguageWizardChooseTitle:         "Локализация",
		KeyLanguageWizardChooseHint:          "Выберите языковой пакет. Он установится один раз для всех серверов; оригинальная папка Morrowind не изменяется.",
		KeyLanguageWizardRussianHint:         "Скачивает русский текст; озвучка добавляется отдельно и необязательна. Кириллические шрифты установятся автоматически. Большая загрузка.",
		KeyLanguageWizardRemoveHint:          "Языковой overlay будет удалён; все профили вернутся к оригинальному языку Morrowind.",
		KeyLanguageWizardInstallTitle:        "Установка локализации",
		KeyLanguageWizardInstallHint:         "Не закрывайте клиент до завершения установки.",
		KeyLanguageWizardCurrent:             "Сейчас: %s",
		KeyLanguageWizardStateMissing:        "не установлена",
		KeyLanguageWizardStateInstalled:      "%s, установлена",
		KeyLanguageWizardStateInstalledVoice: "%s, установлена с озвучкой",
		KeyLanguageWizardRemove:              "Удалить локализацию",
		KeyLanguageWizardApplying:            "Установка локализации...",
		KeyLanguageWizardDone:                "Локализация установлена для всех серверов.",

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
		KeyErrorLocaleMissingAct:       "Откройте «Меню → Локализация» и установите пакет заново.",
		KeyErrorGameMissing:            "TES3MP не установлен.",
		KeyErrorGameMissingAct:         "Откройте мастер подготовки, установите TES3MP и повторите.",
		KeyErrorGameProton:             "Proton не удаётся использовать.",
		KeyErrorGameProtonAct:          "Выберите рабочую папку Proton в Настройках или установите Proton через Steam.",
		KeyErrorGameConfig:             "Не удалось подготовить конфигурацию игры.",
		KeyErrorGameConfigAct:          "Проверьте права на запись в рабочей папке и папке конфигурации OpenMW, затем повторите.",
		KeyErrorGameStart:              "Не удалось запустить TES3MP.",
		KeyErrorGameStartAct:           "Проверьте установку TES3MP, закройте другие копии игры и повторите.",
		KeyErrorGameExited:             "TES3MP неожиданно завершился.",
		KeyErrorGameExitedAct:          "Откройте лог клиента, найдите ошибку игры и повторите после исправления.",

		KeyLogsTitle:      "Лог клиента",
		KeyLogsEmpty:      "Лог пока пуст.",
		KeyLogsRefresh:    "Обновить",
		KeyLogsOpenFolder: "Открыть папку с логом",
		KeyLogsHint:       "Показаны последние %d строк. Полный файл лежит в папке с логом.",
		KeyLogsExitHint:   "Если игра закрылась неожиданно, откройте лог клиента.",

		KeyWizardTitle:             "Первая настройка",
		KeyWizardStep:              "Шаг %d из %d",
		KeyWizardCancel:            "Отменить настройку",
		KeyWizardBack:              "Назад",
		KeyWizardNext:              "Далее",
		KeyWizardFinish:            "Готово",
		KeyWizardRetry:             "Повторить",
		KeyWizardMorrowindTitle:    "Morrowind",
		KeyWizardMorrowindHint:     "Выберите папку с оригинальной игрой. Клиент только читает её.",
		KeyWizardFontsTitle:        "Шрифты",
		KeyWizardFontsHint:         "Без этого набора игра показывает квадратики вместо кириллицы. Набор скачивается один раз и перепроверяется перед каждым запуском; неудачная загрузка не блокирует игру.",
		KeyWizardFontsDownloading:  "Загрузка набора шрифтов...",
		KeyWizardFontsFailed:       "Часть шрифтов не скачалась: %s. Игра запустится со стандартным шрифтом, повтор — при следующем Play.",
		KeyWizardUIVideoTitle:      "Интерфейс и видео",
		KeyWizardUIVideoHint:       "Необязательный шаг: задайте размер текста и FPS или оставьте значения как есть.",
		KeyWizardLocalizationTitle: "Локализация (необязательно)",
		KeyWizardLocalizationHint:  "Установите языковой пакет сразу для всех серверов. Шаг можно пропустить и установить позже с экрана «Installation».",
		KeyWizardTES3MPTitle:       "TES3MP",
		KeyWizardTES3MPHint:        "Клиент скачает и проверит TES3MP %s. Адрес сервера для этого шага не нужен.",
		KeyWizardReadyTitle:        "Можно играть",
		KeyWizardReadyHint:         "Настройка завершена. Добавьте сервер в главном окне, когда захотите играть.",
		KeyWizardSummary:           "Morrowind: %s\nРабочая папка: %s\nTES3MP: %s\nЛог: %s",

		KeySetupFind:                "Найти автоматически",
		KeySetupChoose:              "Выбрать папку",
		KeySetupCheck:               "Проверить файлы",
		KeySetupMorrowindReady:      "Morrowind готов: %s",
		KeySetupMorrowindIncomplete: "Нельзя использовать эту папку: %s",
		KeySetupTES3MP:              "TES3MP %s",
		KeySetupTES3MPReady:         "TES3MP %s установлен.",
		KeySetupTES3MPOutdated:      "Установлен TES3MP %s, нужен %s.",
		KeySetupTES3MPMissing:       "TES3MP ещё не установлен.",
		KeySetupInstall:             "Установить TES3MP",
		KeySetupDownloading:         "Загрузка TES3MP: %s из %s",
		KeySetupVerifying:           "Проверка архива TES3MP...",
		KeySetupExtracting:          "Распаковка TES3MP...",
		KeySetupInstalling:          "Завершение установки TES3MP...",
		KeySetupDone:                "TES3MP установлен и готов.",
		KeySetupLocalizationInstall: "Установить локализацию",
		KeySetupLocalizationReady:   "Локализация установлена: %s",
		KeySetupLocalizationMissing: "Локализация не установлена. Этот шаг можно пропустить.",

		KeyAboutTitle: "О клиенте",
		KeyAboutBody:  "Клиент tes3mp-easy %s\nTES3MP %s\n%s",
		KeyFooterPath: "Конфиг: %s",

		KeyLanguageRussian: "Русский",
	},
}
