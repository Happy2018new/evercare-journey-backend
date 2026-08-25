package define

type LangFormat struct {
	LangKey  string   `json:"lang_key"`
	LangArgs []string `json:"lang_args"`
}

func NewLangFormat(key string, args ...string) *LangFormat {
	return &LangFormat{
		LangKey:  key,
		LangArgs: args,
	}
}

const (
	LangKeyGeneralUnknownErr        = "error.general.unknown"                 // 未知错误
	LangKeyGeneralInvalidRequest    = "error.general.invalid.request"         // 无效请求
	LangKeyGeneralInvalidSession    = "error.general.invalid.session"         // 无效的用户登录状态，请重新启动程序再试
	LangKeyGeneralNameInvalidLen    = "error.general.name.invalid.length"     // 用户名长度应在 3 到 14 个字符之间
	LangKeyGeneralNameInvalidChar   = "error.general.name.invalid.character"  // 用户名只能包含字母、数字和下划线
	LangKeyGeneralSmsGenFailErr     = "error.general.sms.generate.failed"     // 生成短信验证码时发生未知错误
	LangKeyGeneralCaptchaGenFailErr = "error.general.captcha.generate.failed" // 生成图形验证码时发生未知错误

	LangKeyUserCreateUnknownErr         = "error.user.create.unknown"             // 创建用户时发生未知错误
	LangKeyUserCreatePhoneUsedErr       = "error.user.create.phone.used"          // 目标手机号已被注册
	LangKeyUserQueryUnknownErr          = "error.user.query.unknown"              // 查询用户时发生未知错误
	LangKeyUserQueryNotFoundErr         = "error.user.query.not.found"            // 目标用户不存在
	LangKeyUserUpdateProfileFailErr     = "error.user.update.profile.failed"      // 更新用户资料失败
	LangKeyUserUpdateLockSessionFailErr = "error.user.update.lock.session.failed" // 更新用户信息时锁定会话信息失败
	LangKeyUserUpdateLockProfileFailErr = "error.user.update.lock.profile.failed" // 更新用户信息时锁定用户资料失败
	LangKeyUserUpdateUnknownErr         = "error.user.update.unknown"             // 更新用户信息时发生未知错误
	LangKeyUserUpdateLoginTokenErr      = "error.user.update.login.token"         // 更新登录令牌时发生未知错误
	LangKeyUserSessionExtendErr         = "error.user.session.extend"             // 更新会话过期时间时发生未知错误

	LangKeyLoginPhoneLengthErr    = "error.login.phone.length"            // 手机号的长度必须为 %s 位
	LangKeyLoginPhoneInvalidErr   = "error.login.phone.invalid"           // 提供的手机号的格式无效
	LangKeyLoginSmsSendFailErr    = "error.login.sms.send.failed"         // 发送短信验证码时出现未知错误
	LangKeyLoginSmsTranExpiredErr = "error.login.sms.transaction.expired" // 短信验证码已过期，请重新登录

	LangKeyAvatarZipFailErr      = "error.avatar.zip.failed"     // 压缩头像数据失败
	LangKeyAvatarUnzipFailErr    = "error.avatar.unzip.failed"   // 解压头像数据失败
	LangKeyAvatarReachMaxSizeErr = "error.avatar.reach.max.size" // 上传的头像图片不得超过 %s MB
	LangKeyAvatarInvalidData     = "error.avatar.invalid.data"   // 头像图片格式无效
	LangKeyAvatarConvertFailErr  = "error.avatar.convert.failed" // 处理头像图片失败
	LangKeyAvatarSaveFailErr     = "error.avatar.save.failed"    // 保存头像图片失败
	LangKeyAvatarUpdateFailErr   = "error.avatar.update.failed"  // 更新用户头像失败

	LangKeyPlaceNameInvalidErr    = "error.place.name.invalid"    // 给出的地点名称不得为空或超过 %s 个字符
	LangKeyPlaceSaveUnknownErr    = "error.place.save.unknown"    // 保存地点信息时出现未知错误
	LangKeyPlaceQueryUnknownErr   = "error.place.query.unknown"   // 查询地点信息时出现未知错误
	LangKeyPlaceRefreshUnknownErr = "error.place.refresh.unknown" // 刷新地点信息时出现未知错误

	LangKeyTripCreateUnknownErr = "error.trip.create.unknown"  // 创建行程时出现未知错误
	LangKeyTripQueryNotFoundErr = "error.trip.query.not.found" // 目标行程不存在
	LangKeyTripQueryUnknownErr  = "error.trip.query.unknown"   // 查询行程信息时出现未知错误
	LangKeyTripUpdateUnknownErr = "error.trip.update.unknown"  // 更新行程信息时出现未知错误
	LangKeyTripDeleteUnknownErr = "error.trip.delete.unknown"  // 删除行程信息时出现未知错误

	LangKeyTripNodeSaveUnknownErr   = "error.trip.node.save.unknown"   // 保存行程节点信息时出现未知错误
	LangKeyTripNodeDeleteUnknownErr = "error.trip.node.delete.unknown" // 删除行程节点信息时出现未知错误
)
