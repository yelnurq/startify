package mail

// Template identifies a letter.
type Template string

// Letters the platform sends.
const (
	TemplateVerifyEmail   Template = "verify_email"
	TemplateLoginCode     Template = "login_code"
	TemplatePasswordReset Template = "password_reset"
	TemplateInvite        Template = "invite"
	TemplateTeamResponse  Template = "team_response"
)

// copyText is the wording of a letter in one language. Placeholders in
// braces ({name}, {tenant}, {link}, {code}, {minutes}, {responder},
// {request}, {message}) are filled from the job data.
type copyText struct {
	Subject    string
	Preheader  string
	Heading    string
	Paragraphs []string
	// Code is shown in a large box when set (sign-in codes).
	Code   string
	Button string
	// After follows the button: validity, "ignore if it was not you".
	After string
}

type letterCopy map[string]copyText

var catalog = map[Template]letterCopy{
	TemplateVerifyEmail: {
		"kk": {
			Subject:    "Qadam: электрондық поштаңызды растаңыз",
			Preheader:  "Тіркелуді аяқтау үшін бір рет басыңыз",
			Heading:    "Қош келдіңіз, {name}!",
			Paragraphs: []string{"Сіз {tenant} Qadam платформасында тіркелдіңіз. Тіркелуді аяқтау үшін электрондық поштаңызды растаңыз."},
			Button:     "Поштаны растау",
			After:      "Сілтеме 24 сағат жарамды. Егер сіз тіркелмеген болсаңыз, бұл хатты елемеңіз.",
		},
		"ru": {
			Subject:    "Qadam: подтвердите электронную почту",
			Preheader:  "Один клик, чтобы завершить регистрацию",
			Heading:    "Добро пожаловать, {name}!",
			Paragraphs: []string{"Вы зарегистрировались на платформе Qadam — {tenant}. Подтвердите адрес электронной почты, чтобы завершить регистрацию."},
			Button:     "Подтвердить почту",
			After:      "Ссылка действует 24 часа. Если вы не регистрировались, просто проигнорируйте это письмо.",
		},
		"en": {
			Subject:    "Qadam: confirm your e-mail",
			Preheader:  "One click to finish signing up",
			Heading:    "Welcome, {name}!",
			Paragraphs: []string{"You have signed up for Qadam at {tenant}. Confirm your e-mail address to finish registration."},
			Button:     "Confirm e-mail",
			After:      "The link is valid for 24 hours. If you did not sign up, ignore this letter.",
		},
	},
	TemplateLoginCode: {
		"kk": {
			Subject:    "Qadam-ға кіру коды: {code}",
			Preheader:  "Код {minutes} минут жарамды",
			Heading:    "Кіру коды",
			Paragraphs: []string{"{tenant} Qadam платформасына кіру үшін осы кодты енгізіңіз:"},
			Code:       "{code}",
			After:      "Код {minutes} минут жарамды. Ешкімге айтпаңыз — Qadam қызметкерлері кодты ешқашан сұрамайды.",
		},
		"ru": {
			Subject:    "Код для входа в Qadam: {code}",
			Preheader:  "Код действует {minutes} минут",
			Heading:    "Код для входа",
			Paragraphs: []string{"Введите этот код, чтобы войти в Qadam — {tenant}:"},
			Code:       "{code}",
			After:      "Код действует {minutes} минут. Никому его не сообщайте — сотрудники Qadam никогда не спрашивают код.",
		},
		"en": {
			Subject:    "Your Qadam sign-in code: {code}",
			Preheader:  "The code is valid for {minutes} minutes",
			Heading:    "Sign-in code",
			Paragraphs: []string{"Enter this code to sign in to Qadam at {tenant}:"},
			Code:       "{code}",
			After:      "The code is valid for {minutes} minutes. Never share it — Qadam staff will never ask for it.",
		},
	},
	TemplatePasswordReset: {
		"kk": {
			Subject:    "Qadam: құпиясөзді қалпына келтіру",
			Preheader:  "Жаңа құпиясөз орнатыңыз",
			Heading:    "Құпиясөзді қалпына келтіру",
			Paragraphs: []string{"{name}, біз {tenant} Qadam платформасындағы аккаунтыңыздың құпиясөзін қалпына келтіру сұрауын алдық."},
			Button:     "Жаңа құпиясөз орнату",
			After:      "Сілтеме 1 сағат жарамды. Егер сұрауды сіз жібермесеңіз, бұл хатты елемеңіз — құпиясөзіңіз өзгермейді.",
		},
		"ru": {
			Subject:    "Qadam: восстановление пароля",
			Preheader:  "Установите новый пароль",
			Heading:    "Восстановление пароля",
			Paragraphs: []string{"{name}, мы получили запрос на смену пароля вашего аккаунта Qadam — {tenant}."},
			Button:     "Установить новый пароль",
			After:      "Ссылка действует 1 час. Если вы не запрашивали смену пароля, проигнорируйте письмо — пароль останется прежним.",
		},
		"en": {
			Subject:    "Qadam: reset your password",
			Preheader:  "Set a new password",
			Heading:    "Reset your password",
			Paragraphs: []string{"{name}, we received a request to reset the password of your Qadam account at {tenant}."},
			Button:     "Set a new password",
			After:      "The link is valid for 1 hour. If you did not ask for it, ignore this letter — your password stays the same.",
		},
	},
	TemplateInvite: {
		"kk": {
			Subject:   "{tenant} сізді Qadam платформасына шақырады",
			Preheader: "Құпиясөз ойлап табыңыз — және бастаңыз",
			Heading:   "Сәлем, {name}!",
			Paragraphs: []string{
				"{tenant} сізге Qadam платформасында аккаунт ашты. Мұнда AI-ментормен бірге идеяңызды жобаға айналдырып, оны витринада көрсетіп, байқауларға қатыса аласыз.",
				"Бастау үшін құпиясөз ойлап табыңыз.",
			},
			Button: "Құпиясөз орнату",
			After:  "Шақыру 7 күн жарамды.",
		},
		"ru": {
			Subject:   "{tenant} приглашает вас в Qadam",
			Preheader: "Придумайте пароль — и начинайте",
			Heading:   "Здравствуйте, {name}!",
			Paragraphs: []string{
				"{tenant} создал для вас аккаунт на платформе Qadam. Здесь вместе с AI-ментором вы превратите идею в проект, покажете его на витрине и поучаствуете в конкурсах.",
				"Чтобы начать, придумайте пароль.",
			},
			Button: "Установить пароль",
			After:  "Приглашение действует 7 дней.",
		},
		"en": {
			Subject:   "{tenant} invites you to Qadam",
			Preheader: "Choose a password and get started",
			Heading:   "Hello, {name}!",
			Paragraphs: []string{
				"{tenant} has created a Qadam account for you. Here you will turn an idea into a project with an AI mentor, present it on the showcase and take part in competitions.",
				"To get started, choose a password.",
			},
			Button: "Set a password",
			After:  "The invitation is valid for 7 days.",
		},
	},
	TemplateTeamResponse: {
		"kk": {
			Subject:    "«{request}» сұрауыңызға жауап келді",
			Preheader:  "{responder} командаға қосылғысы келеді",
			Heading:    "Жаңа жауап",
			Paragraphs: []string{"{responder} сіздің «{request}» сұрауыңызға жауап берді:", "«{message}»"},
			Button:     "Жауапты ашу",
			After:      "Байланыс деректері Qadam-дағы сұрау бетінде.",
		},
		"ru": {
			Subject:    "Отклик на ваш запрос «{request}»",
			Preheader:  "{responder} хочет присоединиться к команде",
			Heading:    "Новый отклик",
			Paragraphs: []string{"{responder} откликнулся на ваш запрос «{request}»:", "«{message}»"},
			Button:     "Открыть отклик",
			After:      "Контакты — на странице запроса в Qadam.",
		},
		"en": {
			Subject:    "A reply to your request “{request}”",
			Preheader:  "{responder} would like to join your team",
			Heading:    "New reply",
			Paragraphs: []string{"{responder} replied to your request “{request}”:", "“{message}”"},
			Button:     "Open the reply",
			After:      "Contact details are on the request page in Qadam.",
		},
	},
}
