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
	KeyCommonSettings = "common.settings"
	KeyCommonAbout    = "common.about"
	KeyCommonPlay     = "common.play"
	KeyCommonLanguage = "common.language"
	KeyCommonConfirm  = "common.confirm"
	KeyCommonError    = "common.error"
	KeyCommonSetup    = "common.setup"
	KeyCommonAdvanced = "common.advanced"

	KeyServerEmptyTitle  = "server.empty.title"
	KeyServerEmptyHint   = "server.empty.hint"
	KeyServerActive      = "server.active"
	KeyServerMakeActive  = "server.make_active"
	KeyServerPanel       = "server.panel"
	KeyServerPlayTitle   = "server.play.title"
	KeyServerPlayBody    = "server.play.body"
	KeyServerAddTitle    = "server.add.title"
	KeyServerEditTitle   = "server.edit.title"
	KeyServerDeleteTitle = "server.delete.title"
	KeyServerDeleteBody  = "server.delete.body"
	KeyServerFieldName   = "server.field.name"
	KeyServerFieldAddr   = "server.field.address"
	KeyServerFieldPort   = "server.field.port"
	KeyServerFieldPass   = "server.field.password"
	KeyServerFieldPanel  = "server.field.panel_url"
	KeyServerCheck       = "server.check"
	KeyServerCheckTitle  = "server.check.title"
	KeyServerCheckBody   = "server.check.body"
	KeyServerUpdateMods  = "server.update_mods"
	KeyServerLaunching   = "server.launching"
	KeyServerLaunched    = "server.launched"
	KeyServerWarnings    = "server.warnings"
	KeyServerExited      = "server.exited"
	KeyServerExitFailed  = "server.exit_failed"

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
	KeyModsFailed      = "mods.failed"

	KeySettingsTitle            = "settings.title"
	KeySettingsMorrowind        = "settings.morrowind"
	KeySettingsWorkDir          = "settings.work_dir"
	KeySettingsProton           = "settings.proton"
	KeySettingsHint             = "settings.hint"
	KeySettingsFontsHint        = "settings.fonts_hint"
	KeySettingsFonts            = "settings.fonts"
	KeySettingsLocalization     = "settings.localization"
	KeySettingsVoiceover        = "settings.voiceover"
	KeySettingsNoServer         = "settings.no_server"
	KeySettingsLocalizationHint = "settings.localization_hint"
	KeySettingsTheme            = "settings.theme"

	KeyThemeSystem = "theme.system"
	KeyThemeDark   = "theme.dark"
	KeyThemeLight  = "theme.light"

	KeyLocaleTitle       = "locale.title"
	KeyLocaleChecking    = "locale.checking"
	KeyLocaleDownloading = "locale.downloading"
	KeyLocaleExtracting  = "locale.extracting"
	KeyLocaleInstalling  = "locale.installing"
	KeyLocaleDone        = "locale.done"
	KeyLocaleRemoved     = "locale.removed"
	KeyLocaleFailed      = "locale.failed"

	KeyAdvancedTitle   = "advanced.title"
	KeyAdvancedOpenCS  = "advanced.open_cs"
	KeyAdvancedCSHint  = "advanced.cs_hint"
	KeyAdvancedStarted = "advanced.started"
	KeyAdvancedFailed  = "advanced.failed"

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
	KeyLogsPath       = "logs.path"
	KeyLogsEmpty      = "logs.empty"
	KeyLogsRefresh    = "logs.refresh"
	KeyLogsOpenFolder = "logs.open_folder"
	KeyLogsHint       = "logs.hint"
	KeyLogsExitHint   = "logs.exit_hint"

	KeySettingsOpenLog = "settings.open_log"
	KeySettingsLogHint = "settings.log_hint"

	KeyWizardTitle              = "wizard.title"
	KeyWizardStep               = "wizard.step"
	KeyWizardCancel             = "wizard.cancel"
	KeyWizardBack               = "wizard.back"
	KeyWizardNext               = "wizard.next"
	KeyWizardFinish             = "wizard.finish"
	KeyWizardRetry              = "wizard.retry"
	KeyWizardLanguageTitle      = "wizard.language.title"
	KeyWizardLanguageHint       = "wizard.language.hint"
	KeyWizardMorrowindTitle     = "wizard.morrowind.title"
	KeyWizardMorrowindHint      = "wizard.morrowind.hint"
	KeyWizardWorkDirTitle       = "wizard.work_dir.title"
	KeyWizardWorkDirHint        = "wizard.work_dir.hint"
	KeyWizardServerTitle        = "wizard.server.title"
	KeyWizardServerHint         = "wizard.server.hint"
	KeyWizardConnectionTitle    = "wizard.connection.title"
	KeyWizardConnectionHint     = "wizard.connection.hint"
	KeyWizardConnectionChecking = "wizard.connection.checking"
	KeyWizardConnectionOK       = "wizard.connection.ok"
	KeyWizardConnectionNeeded   = "wizard.connection.needed"
	KeyWizardTES3MPNeeded       = "wizard.connection.tes3mp_needed"
	KeyWizardTES3MPTitle        = "wizard.tes3mp.title"
	KeyWizardTES3MPHint         = "wizard.tes3mp.hint"
	KeyWizardFontsTitle         = "wizard.fonts.title"
	KeyWizardFontsHint          = "wizard.fonts.hint"
	KeyWizardModsTitle          = "wizard.mods.title"
	KeyWizardModsHint           = "wizard.mods.hint"
	KeyWizardModsStarting       = "wizard.mods.starting"
	KeyWizardModsReady          = "wizard.mods.ready"
	KeyWizardLocaleTitle        = "wizard.locale.title"
	KeyWizardLocaleHint         = "wizard.locale.hint"
	KeyWizardFontsChecking      = "wizard.fonts.checking"
	KeyWizardFontsReady         = "wizard.fonts.ready"
	KeyWizardFontsWarning       = "wizard.fonts.warning"
	KeyWizardFontsSkipped       = "wizard.fonts.skipped"
	KeyWizardLocaleInstall      = "wizard.locale.install"
	KeyWizardLocaleInstalled    = "wizard.locale.installed"
	KeyWizardLocaleSkipped      = "wizard.locale.skipped"
	KeyWizardLocaleNeeded       = "wizard.locale.needed"
	KeyWizardReadyTitle         = "wizard.ready.title"
	KeyWizardReadyHint          = "wizard.ready.hint"
	KeyWizardSummary            = "wizard.ready.summary"
	KeyWizardPlay               = "wizard.ready.play"

	KeySetupTitle               = "setup.title"
	KeySetupMorrowind           = "setup.morrowind"
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
	KeySetupFailed              = "setup.failed"

	KeyAboutTitle = "about.title"
	KeyAboutBody  = "about.body"
	KeyFooterPath = "footer.config_path"

	KeyLanguageEnglish = "lang.english"
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
		KeyCommonSettings: "Settings",
		KeyCommonAbout:    "About",
		KeyCommonPlay:     "Play",
		KeyCommonLanguage: "Language",
		KeyCommonConfirm:  "Confirm",
		KeyCommonError:    "Error",
		KeyCommonSetup:    "Prepare",
		KeyCommonAdvanced: "Advanced",

		KeyServerEmptyTitle:  "No servers yet",
		KeyServerEmptyHint:   "Add the server address your friends sent you.",
		KeyServerActive:      "active",
		KeyServerMakeActive:  "Make active",
		KeyServerPanel:       "Panel",
		KeyServerPlayTitle:   "Launch is not available yet",
		KeyServerPlayBody:    "The launch scenario will be added in the next client steps.",
		KeyServerAddTitle:    "Add server",
		KeyServerEditTitle:   "Edit server",
		KeyServerDeleteTitle: "Delete server",
		KeyServerDeleteBody:  "Delete the profile %q? Files already downloaded stay on disk.",
		KeyServerFieldName:   "Name",
		KeyServerFieldAddr:   "Address",
		KeyServerFieldPort:   "Port",
		KeyServerFieldPass:   "Password",
		KeyServerFieldPanel:  "Panel URL",
		KeyServerCheck:       "Check server",
		KeyServerCheckTitle:  "Server status",
		KeyServerCheckBody:   "Server: %s\nAddress: %s:%d\nStatus: %s\nMods: %s",
		KeyServerUpdateMods:  "Update mods",
		KeyServerLaunching:   "Preparing the launch...",
		KeyServerLaunched:    "The game is running.",
		KeyServerWarnings:    "Warnings:\n%s",
		KeyServerExited:      "The game has exited.",
		KeyServerExitFailed:  "The game exited with an error: %v",

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
		KeyModsFailed:      "Mod update failed: %v",

		KeySettingsTitle:            "Settings",
		KeySettingsTheme:            "Theme",
		KeySettingsMorrowind:        "Morrowind folder",
		KeySettingsWorkDir:          "Work folder",
		KeySettingsProton:           "Proton (Linux)",
		KeySettingsHint:             "Leave a path empty to use the default.",
		KeySettingsFontsHint:        "Cyrillic fonts are installed before launch. Russian localization always enables them.",
		KeySettingsFonts:            "Cyrillic fonts",
		KeySettingsLocalization:     "Russian localization",
		KeySettingsVoiceover:        "Russian voiceover (large download)",
		KeySettingsNoServer:         "Add a server profile to install the localization.",
		KeySettingsLocalizationHint: "The localization is copied into the selected server's Data Files; the original Morrowind folder is never modified.",

		KeyThemeSystem: "Follow system",
		KeyThemeDark:   "Amber dark",
		KeyThemeLight:  "Amber light",

		KeyLocaleTitle:       "Russian localization",
		KeyLocaleChecking:    "Checking the installed localization...",
		KeyLocaleDownloading: "Downloading localization: %s of %s",
		KeyLocaleExtracting:  "Extracting localization...",
		KeyLocaleInstalling:  "Applying localization...",
		KeyLocaleDone:        "The Russian localization is installed.",
		KeyLocaleRemoved:     "The Russian localization overlay was removed.",
		KeyLocaleFailed:      "Localization failed: %v",

		KeyAdvancedTitle:   "Advanced",
		KeyAdvancedOpenCS:  "Run OpenMW-CS",
		KeyAdvancedCSHint:  "OpenMW-CS is not bundled with TES3MP. If it is missing, download it from https://openmw.org/downloads/ and put openmw-cs.exe next to tes3mp.exe.",
		KeyAdvancedStarted: "OpenMW-CS is running.",
		KeyAdvancedFailed:  "OpenMW-CS failed to start: %v",

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
		KeyErrorLocaleMissing:          "Russian localization is enabled but not installed.",
		KeyErrorLocaleMissingAct:       "Open Settings, install the localization again or disable it.",
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
		KeyLogsPath:       "File: %s",
		KeyLogsEmpty:      "The log is empty yet.",
		KeyLogsRefresh:    "Refresh",
		KeyLogsOpenFolder: "Open log folder",
		KeyLogsHint:       "The last %d lines are shown. The full file is in the log folder.",
		KeyLogsExitHint:   "If the game closed unexpectedly, open the client log.",

		KeySettingsOpenLog: "Client log",
		KeySettingsLogHint: "Open the log if setup or a launch fails.",

		KeyWizardTitle:              "First-run setup",
		KeyWizardStep:               "Step %d of %d",
		KeyWizardCancel:             "Cancel setup",
		KeyWizardBack:               "Back",
		KeyWizardNext:               "Next",
		KeyWizardFinish:             "Finish",
		KeyWizardRetry:              "Retry",
		KeyWizardLanguageTitle:      "Interface language",
		KeyWizardLanguageHint:       "Choose the language. You can change it later in Settings.",
		KeyWizardMorrowindTitle:     "Morrowind",
		KeyWizardMorrowindHint:      "Select the original game folder. The client only reads it.",
		KeyWizardWorkDirTitle:       "Work folder",
		KeyWizardWorkDirHint:        "TES3MP, mods and logs are stored here. Leave it empty to use the default folder.",
		KeyWizardServerTitle:        "Server",
		KeyWizardServerHint:         "Enter the address and panel URL you received from the server owner.",
		KeyWizardConnectionTitle:    "Connection and TES3MP",
		KeyWizardConnectionHint:     "The client checks the panel and the installed TES3MP build.",
		KeyWizardConnectionChecking: "Checking the panel...",
		KeyWizardConnectionOK:       "The panel is reachable. Server status: %s.",
		KeyWizardConnectionNeeded:   "The panel must be reachable before continuing.",
		KeyWizardTES3MPNeeded:       "TES3MP %s must be installed before continuing.",
		KeyWizardTES3MPTitle:        "TES3MP",
		KeyWizardTES3MPHint:         "The client downloads and verifies TES3MP %s. No server details are needed for this step.",
		KeyWizardFontsTitle:         "Fonts",
		KeyWizardFontsHint:          "Cyrillic fonts are installed into the OpenMW configuration. The Morrowind folder is not modified.",
		KeyWizardModsTitle:          "Server mods",
		KeyWizardModsHint:           "The client downloads and verifies the current mod pack.",
		KeyWizardModsStarting:       "Installing server mods...",
		KeyWizardModsReady:          "Server mods are installed.",
		KeyWizardLocaleTitle:        "Fonts and localization",
		KeyWizardLocaleHint:         "Cyrillic fonts are installed automatically. Russian localization is optional.",
		KeyWizardFontsChecking:      "Installing Cyrillic fonts...",
		KeyWizardFontsReady:         "Cyrillic fonts are ready.",
		KeyWizardFontsWarning:       "Fonts could not be installed: %s. The game will use the default font.",
		KeyWizardFontsSkipped:       "Cyrillic fonts are disabled. You can enable them in Settings.",
		KeyWizardLocaleInstall:      "Install Russian localization",
		KeyWizardLocaleInstalled:    "Russian localization is installed.",
		KeyWizardLocaleSkipped:      "Russian localization is not installed.",
		KeyWizardLocaleNeeded:       "Russian localization is not installed yet.",
		KeyWizardReadyTitle:         "Ready to play",
		KeyWizardReadyHint:          "Setup is complete. Add a server from the main window when you are ready to play.",
		KeyWizardSummary:            "Morrowind: %s\nWork folder: %s\nTES3MP: %s\nLog: %s",
		KeyWizardPlay:               "Play",

		KeySetupTitle:               "Prepare the client",
		KeySetupMorrowind:           "Morrowind",
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
		KeySetupFailed:              "Setup failed: %v",

		KeyAboutTitle: "About",
		KeyAboutBody:  "tes3mp-easy client %s\nTES3MP %s\n%s",
		KeyFooterPath: "Config: %s",

		KeyLanguageEnglish: "English",
		KeyLanguageRussian: "Русский",
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
		KeyCommonSettings: "Настройки",
		KeyCommonAbout:    "О клиенте",
		KeyCommonPlay:     "Играть",
		KeyCommonLanguage: "Язык",
		KeyCommonConfirm:  "Подтвердить",
		KeyCommonError:    "Ошибка",
		KeyCommonSetup:    "Подготовка",
		KeyCommonAdvanced: "Дополнительно",

		KeyServerEmptyTitle:  "Серверов пока нет",
		KeyServerEmptyHint:   "Добавьте адрес сервера, который прислали друзья.",
		KeyServerActive:      "активен",
		KeyServerMakeActive:  "Сделать активным",
		KeyServerPanel:       "Панель",
		KeyServerPlayTitle:   "Запуск пока недоступен",
		KeyServerPlayBody:    "Сценарий запуска появится на следующих шагах клиента.",
		KeyServerAddTitle:    "Добавить сервер",
		KeyServerEditTitle:   "Изменить сервер",
		KeyServerDeleteTitle: "Удалить сервер",
		KeyServerDeleteBody:  "Удалить профиль %q? Уже скачанные файлы останутся на диске.",
		KeyServerFieldName:   "Название",
		KeyServerFieldAddr:   "Адрес",
		KeyServerFieldPort:   "Порт",
		KeyServerFieldPass:   "Пароль",
		KeyServerFieldPanel:  "URL панели",
		KeyServerCheck:       "Проверить сервер",
		KeyServerCheckTitle:  "Статус сервера",
		KeyServerCheckBody:   "Сервер: %s\nАдрес: %s:%d\nСтатус: %s\nМоды: %s",
		KeyServerUpdateMods:  "Обновить моды",
		KeyServerLaunching:   "Подготовка к запуску...",
		KeyServerLaunched:    "Игра запущена.",
		KeyServerWarnings:    "Предупреждения:\n%s",
		KeyServerExited:      "Игра закрыта.",
		KeyServerExitFailed:  "Игра завершилась с ошибкой: %v",

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
		KeyModsFailed:      "Ошибка обновления модов: %v",

		KeySettingsTitle:            "Настройки",
		KeySettingsTheme:            "Тема",
		KeySettingsMorrowind:        "Папка Morrowind",
		KeySettingsWorkDir:          "Рабочая папка",
		KeySettingsProton:           "Proton (Linux)",
		KeySettingsHint:             "Пустое поле означает путь по умолчанию.",
		KeySettingsFontsHint:        "Кириллические шрифты ставятся перед запуском. Русская локализация всегда включает их.",
		KeySettingsFonts:            "Кириллические шрифты",
		KeySettingsLocalization:     "Русская локализация",
		KeySettingsVoiceover:        "Русская озвучка (большая загрузка)",
		KeySettingsNoServer:         "Добавьте сервер, чтобы установить локализацию.",
		KeySettingsLocalizationHint: "Локализация копируется в Data Files выбранного сервера; папка Morrowind не изменяется.",

		KeyThemeSystem: "Как в системе",
		KeyThemeDark:   "Янтарная тёмная",
		KeyThemeLight:  "Янтарная светлая",

		KeyLocaleTitle:       "Русская локализация",
		KeyLocaleChecking:    "Проверка установленной локализации...",
		KeyLocaleDownloading: "Загрузка локализации: %s из %s",
		KeyLocaleExtracting:  "Распаковка локализации...",
		KeyLocaleInstalling:  "Применение локализации...",
		KeyLocaleDone:        "Русская локализация установлена.",
		KeyLocaleRemoved:     "Русская локализация удалена из профиля.",
		KeyLocaleFailed:      "Ошибка локализации: %v",

		KeyAdvancedTitle:   "Дополнительно",
		KeyAdvancedOpenCS:  "Запустить OpenMW-CS",
		KeyAdvancedCSHint:  "OpenMW-CS не входит в TES3MP. Если его нет, скачайте с https://openmw.org/downloads/ и положите openmw-cs.exe рядом с tes3mp.exe.",
		KeyAdvancedStarted: "OpenMW-CS запущен.",
		KeyAdvancedFailed:  "Не удалось запустить OpenMW-CS: %v",

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
		KeyErrorLocaleMissing:          "Русская локализация включена, но не установлена.",
		KeyErrorLocaleMissingAct:       "Откройте Настройки, установите локализацию заново или отключите её.",
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
		KeyLogsPath:       "Файл: %s",
		KeyLogsEmpty:      "Лог пока пуст.",
		KeyLogsRefresh:    "Обновить",
		KeyLogsOpenFolder: "Открыть папку с логом",
		KeyLogsHint:       "Показаны последние %d строк. Полный файл лежит в папке с логом.",
		KeyLogsExitHint:   "Если игра закрылась неожиданно, откройте лог клиента.",

		KeySettingsOpenLog: "Лог клиента",
		KeySettingsLogHint: "Откройте лог, если настройка или запуск не удались.",

		KeyWizardTitle:              "Первая настройка",
		KeyWizardStep:               "Шаг %d из %d",
		KeyWizardCancel:             "Отменить настройку",
		KeyWizardBack:               "Назад",
		KeyWizardNext:               "Далее",
		KeyWizardFinish:             "Готово",
		KeyWizardRetry:              "Повторить",
		KeyWizardLanguageTitle:      "Язык интерфейса",
		KeyWizardLanguageHint:       "Выберите язык. Его можно изменить позже в Настройках.",
		KeyWizardMorrowindTitle:     "Morrowind",
		KeyWizardMorrowindHint:      "Выберите папку с оригинальной игрой. Клиент только читает её.",
		KeyWizardWorkDirTitle:       "Рабочая папка",
		KeyWizardWorkDirHint:        "Здесь будут TES3MP, моды и логи. Оставьте пустой, чтобы использовать папку по умолчанию.",
		KeyWizardServerTitle:        "Сервер",
		KeyWizardServerHint:         "Введите адрес и адрес панели, которые вам дал владелец сервера.",
		KeyWizardConnectionTitle:    "Связь и TES3MP",
		KeyWizardConnectionHint:     "Клиент проверит панель и установленную сборку TES3MP.",
		KeyWizardConnectionChecking: "Проверка панели...",
		KeyWizardConnectionOK:       "Панель доступна. Статус сервера: %s.",
		KeyWizardConnectionNeeded:   "Чтобы продолжить, панель должна быть доступна.",
		KeyWizardTES3MPNeeded:       "Чтобы продолжить, установите TES3MP %s.",
		KeyWizardTES3MPTitle:        "TES3MP",
		KeyWizardTES3MPHint:         "Клиент скачает и проверит TES3MP %s. Адрес сервера для этого шага не нужен.",
		KeyWizardFontsTitle:         "Шрифты",
		KeyWizardFontsHint:          "Кириллические шрифты ставятся в конфигурацию OpenMW. Папка Morrowind не изменяется.",
		KeyWizardModsTitle:          "Моды сервера",
		KeyWizardModsHint:           "Клиент скачает и проверит актуальный набор модов.",
		KeyWizardModsStarting:       "Установка модов сервера...",
		KeyWizardModsReady:          "Моды сервера установлены.",
		KeyWizardLocaleTitle:        "Шрифты и локализация",
		KeyWizardLocaleHint:         "Кириллические шрифты ставятся автоматически. Русская локализация необязательна.",
		KeyWizardFontsChecking:      "Установка кириллических шрифтов...",
		KeyWizardFontsReady:         "Кириллические шрифты готовы.",
		KeyWizardFontsWarning:       "Не удалось установить шрифты: %s. Игра будет использовать стандартный шрифт.",
		KeyWizardFontsSkipped:       "Кириллические шрифты отключены. Их можно включить в Настройках.",
		KeyWizardLocaleInstall:      "Установить русскую локализацию",
		KeyWizardLocaleInstalled:    "Русская локализация установлена.",
		KeyWizardLocaleSkipped:      "Русская локализация не установлена.",
		KeyWizardLocaleNeeded:       "Русская локализация ещё не установлена.",
		KeyWizardReadyTitle:         "Можно играть",
		KeyWizardReadyHint:          "Настройка завершена. Добавьте сервер в главном окне, когда захотите играть.",
		KeyWizardSummary:            "Morrowind: %s\nРабочая папка: %s\nTES3MP: %s\nЛог: %s",
		KeyWizardPlay:               "Играть",

		KeySetupTitle:               "Подготовка клиента",
		KeySetupMorrowind:           "Morrowind",
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
		KeySetupFailed:              "Ошибка подготовки: %v",

		KeyAboutTitle: "О клиенте",
		KeyAboutBody:  "Клиент tes3mp-easy %s\nTES3MP %s\n%s",
		KeyFooterPath: "Конфиг: %s",

		KeyLanguageEnglish: "English",
		KeyLanguageRussian: "Русский",
	},
}
