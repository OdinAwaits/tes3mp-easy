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
	KeySettingsLocalization     = "settings.localization"
	KeySettingsVoiceover        = "settings.voiceover"
	KeySettingsNoServer         = "settings.no_server"
	KeySettingsLocalizationHint = "settings.localization_hint"

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
		KeySettingsMorrowind:        "Morrowind folder",
		KeySettingsWorkDir:          "Work folder",
		KeySettingsProton:           "Proton (Linux)",
		KeySettingsHint:             "Leave a path empty to use the default.",
		KeySettingsFontsHint:        "Cyrillic fonts are installed automatically before every launch.",
		KeySettingsLocalization:     "Russian localization",
		KeySettingsVoiceover:        "Russian voiceover (large download)",
		KeySettingsNoServer:         "Add a server profile to install the localization.",
		KeySettingsLocalizationHint: "The localization is copied into the selected server's Data Files; the original Morrowind folder is never modified.",

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
		KeySettingsMorrowind:        "Папка Morrowind",
		KeySettingsWorkDir:          "Рабочая папка",
		KeySettingsProton:           "Proton (Linux)",
		KeySettingsHint:             "Пустое поле означает путь по умолчанию.",
		KeySettingsFontsHint:        "Кириллические шрифты ставятся автоматически перед каждым запуском.",
		KeySettingsLocalization:     "Русская локализация",
		KeySettingsVoiceover:        "Русская озвучка (большая загрузка)",
		KeySettingsNoServer:         "Добавьте сервер, чтобы установить локализацию.",
		KeySettingsLocalizationHint: "Локализация копируется в Data Files выбранного сервера; папка Morrowind не изменяется.",

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
