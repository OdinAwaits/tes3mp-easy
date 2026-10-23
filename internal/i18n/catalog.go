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

	KeySettingsTitle     = "settings.title"
	KeySettingsMorrowind = "settings.morrowind"
	KeySettingsWorkDir   = "settings.work_dir"
	KeySettingsProton    = "settings.proton"
	KeySettingsHint      = "settings.hint"

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

		KeySettingsTitle:     "Settings",
		KeySettingsMorrowind: "Morrowind folder",
		KeySettingsWorkDir:   "Work folder",
		KeySettingsProton:    "Proton (Linux)",
		KeySettingsHint:      "Leave a path empty to use the default.",

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

		KeySettingsTitle:     "Настройки",
		KeySettingsMorrowind: "Папка Morrowind",
		KeySettingsWorkDir:   "Рабочая папка",
		KeySettingsProton:    "Proton (Linux)",
		KeySettingsHint:      "Пустое поле означает путь по умолчанию.",

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
