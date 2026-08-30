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
	LangKeyGeneralUnknownErr          = "error.general.unknown"                 // 未知错误
	LangKeyGeneralInvalidRequest      = "error.general.invalid.request"         // 无效请求
	LangKeyGeneralRequestBodyInvalid  = "error.general.request.body.invalid"    // 请求体格式无效
	LangKeyGeneralRequestBodyTooLarge = "error.general.request.body.too.large"  // 请求体过大
	LangKeyGeneralInvalidSession      = "error.general.invalid.session"         // 无效的用户登录状态，请重新启动程序再试
	LangKeyGeneralNameInvalidLen      = "error.general.name.invalid.length"     // 用户名长度应在 3 到 14 个字符之间
	LangKeyGeneralNameInvalidChar     = "error.general.name.invalid.character"  // 用户名只能包含字母、数字和下划线
	LangKeyGeneralSmsGenFailErr       = "error.general.sms.generate.failed"     // 生成短信验证码时发生未知错误
	LangKeyGeneralCaptchaGenFailErr   = "error.general.captcha.generate.failed" // 生成图形验证码时发生未知错误

	LangKeyUserCreateUnknownErr         = "error.user.create.unknown"             // 创建用户时发生未知错误
	LangKeyUserCreatePhoneUsedErr       = "error.user.create.phone.used"          // 目标手机号已被注册
	LangKeyUserQueryUnknownErr          = "error.user.query.unknown"              // 查询用户时发生未知错误
	LangKeyUserQueryNotFoundErr         = "error.user.query.not.found"            // 目标用户不存在
	LangKeyUserUpdateProfileFailErr     = "error.user.update.profile.failed"      // 更新用户资料失败
	LangKeyUserUpdateNameUsedErr        = "error.user.update.name.used"           // 用户名已被其他用户使用
	LangKeyUserUpdateLockSessionFailErr = "error.user.update.lock.session.failed" // 更新用户信息时锁定会话信息失败
	LangKeyUserUpdateLockProfileFailErr = "error.user.update.lock.profile.failed" // 更新用户信息时锁定用户资料失败
	LangKeyUserUpdateUnknownErr         = "error.user.update.unknown"             // 更新用户信息时发生未知错误
	LangKeyUserUpdateLoginTokenErr      = "error.user.update.login.token"         // 更新登录令牌时发生未知错误
	LangKeyUserSessionExtendErr         = "error.user.session.extend"             // 更新会话过期时间时发生未知错误

	LangKeyLoginPhoneLengthErr     = "error.login.phone.length"            // 手机号的长度必须为 %s 位
	LangKeyLoginPhoneInvalidErr    = "error.login.phone.invalid"           // 提供的手机号的格式无效
	LangKeyLoginRequestTypeInvalid = "error.login.request.type.invalid"    // 登录请求类型无效
	LangKeyLoginCaptchaInvalid     = "error.login.captcha.invalid"         // 验证码请求无效
	LangKeyLoginSmsCodeInvalid     = "error.login.sms.code.invalid"        // 短信验证码格式无效
	LangKeyLoginSmsTooManyAttempts = "error.login.sms.too.many.attempts"   // 短信验证码错误次数过多
	LangKeyLoginSmsSendFailErr     = "error.login.sms.send.failed"         // 发送短信验证码时出现未知错误
	LangKeyLoginSmsTranExpiredErr  = "error.login.sms.transaction.expired" // 短信验证码已过期，请重新登录
	LangKeyLoginSessionRefreshErr  = "error.login.session.refresh.failed"  // 刷新登录会话时出现未知错误

	LangKeyAvatarZipFailErr      = "error.avatar.zip.failed"     // 压缩头像数据失败
	LangKeyAvatarUnzipFailErr    = "error.avatar.unzip.failed"   // 解压头像数据失败
	LangKeyAvatarReachMaxSizeErr = "error.avatar.reach.max.size" // 上传的头像图片不得超过 %s MB
	LangKeyAvatarInvalidData     = "error.avatar.invalid.data"   // 头像图片格式无效
	LangKeyAvatarConvertFailErr  = "error.avatar.convert.failed" // 处理头像图片失败
	LangKeyAvatarSaveFailErr     = "error.avatar.save.failed"    // 保存头像图片失败
	LangKeyAvatarUpdateFailErr   = "error.avatar.update.failed"  // 更新用户头像失败
	LangKeyAvatarQueryUnknownErr = "error.avatar.query.unknown"  // 查询头像时出现未知错误

	LangKeyPlaceNameInvalidErr          = "error.place.name.invalid"              // 给出的地点名称不得为空或超过 %s 个字符
	LangKeyPlaceRequestBodyInvalid      = "error.place.request.body.invalid"      // 地点请求体格式无效
	LangKeyPlaceIdentityInvalid         = "error.place.identity.invalid"          // 地点身份格式无效
	LangKeyPlaceAmapIdentityInvalid     = "error.place.amap.identity.invalid"     // 高德地点身份格式无效
	LangKeyPlaceSearchUnknownErr        = "error.place.search.unknown"            // 搜索地点时出现未知错误
	LangKeyPlaceSearchKeywordInvalid    = "error.place.search.keyword.invalid"    // 地点搜索关键词格式无效
	LangKeyPlaceSearchCityInvalid       = "error.place.search.city.invalid"       // 地点搜索城市格式无效
	LangKeyPlaceSearchCategoryInvalid   = "error.place.search.category.invalid"   // 地点搜索分类格式无效
	LangKeyPlaceSearchPageInvalid       = "error.place.search.page.invalid"       // 地点搜索页码无效
	LangKeyPlaceSearchPageSizeInvalid   = "error.place.search.page.size.invalid"  // 地点搜索分页大小无效
	LangKeyPlaceNearbyQueryErr          = "error.place.nearby.unknown"            // 查询附近地点时出现未知错误
	LangKeyPlaceNearbyLongitudeInvalid  = "error.place.nearby.longitude.invalid"  // 附近地点查询经度无效
	LangKeyPlaceNearbyLatitudeInvalid   = "error.place.nearby.latitude.invalid"   // 附近地点查询纬度无效
	LangKeyPlaceNearbyCoordinateInvalid = "error.place.nearby.coordinate.invalid" // 附近地点查询坐标无效
	LangKeyPlaceNearbyRadiusInvalid     = "error.place.nearby.radius.invalid"     // 附近地点查询半径无效
	LangKeyPlaceNearbyKeywordInvalid    = "error.place.nearby.keyword.invalid"    // 附近地点查询关键词格式无效
	LangKeyPlaceNearbySortRuleInvalid   = "error.place.nearby.sort.rule.invalid"  // 附近地点查询排序规则无效
	LangKeyPlaceSaveUnknownErr          = "error.place.save.unknown"              // 保存地点信息时出现未知错误
	LangKeyPlaceQueryNotFoundErr        = "error.place.query.not.found"           // 目标地点不存在
	LangKeyPlaceQueryUnknownErr         = "error.place.query.unknown"             // 查询地点信息时出现未知错误
	LangKeyPlaceRefreshUnknownErr       = "error.place.refresh.unknown"           // 刷新地点信息时出现未知错误
	LangKeyPlaceProviderIdentityInvalid = "error.place.provider.identity.invalid" // 地图服务返回的地点身份无效

	LangKeyTripRequestBodyInvalid         = "error.trip.request.body.invalid"         // 行程请求体格式无效
	LangKeyTripIdentityInvalid            = "error.trip.identity.invalid"             // 行程身份格式无效
	LangKeyTripIdentityListInvalid        = "error.trip.identity.list.invalid"        // 行程身份列表格式无效
	LangKeyTripNameInvalid                = "error.trip.name.invalid"                 // 行程名称格式无效
	LangKeyTripDateInvalid                = "error.trip.date.invalid"                 // 行程日期格式无效
	LangKeyTripTravelModeInvalid          = "error.trip.travel.mode.invalid"          // 行程交通方式无效
	LangKeyTripStatusInvalid              = "error.trip.status.invalid"               // 行程状态无效
	LangKeyTripStatusTransitionInvalid    = "error.trip.status.transition.invalid"    // 行程状态不能这样变更
	LangKeyTripStatusLocked               = "error.trip.status.locked"                // 已开始的行程不可修改规划信息
	LangKeyTripStatusTerminal             = "error.trip.status.terminal"              // 已结束的行程不可继续编辑
	LangKeyTripCompleteNodesIncomplete    = "error.trip.complete.nodes.incomplete"    // 尚有未完成节点，不能结束行程
	LangKeyTripVersionExhausted           = "error.trip.version.exhausted"            // 行程版本号已耗尽
	LangKeyTripVersionConflict            = "error.trip.version.conflict"             // 行程已被其他设备更新
	LangKeyTripStartEndSameInvalid        = "error.trip.start.end.same"               // 行程起点和终点不能相同
	LangKeyTripCreateNameUsedErr          = "error.trip.create.name.used"             // 不允许创建名称相同的行程
	LangKeyTripCreateUnknownErr           = "error.trip.create.unknown"               // 创建行程时出现未知错误
	LangKeyTripQueryNotFoundErr           = "error.trip.query.not.found"              // 目标行程不存在
	LangKeyTripQueryUnknownErr            = "error.trip.query.unknown"                // 查询行程信息时出现未知错误
	LangKeyTripUpdateNameUsedErr          = "error.trip.update.name.used"             // 目标行程的名称已被其他行程使用，请更换名称后再试
	LangKeyTripUpdateUnknownErr           = "error.trip.update.unknown"               // 更新行程信息时出现未知错误
	LangKeyTripDeleteUnknownErr           = "error.trip.delete.unknown"               // 删除行程信息时出现未知错误
	LangKeyTripOptimizeUnknownErr         = "error.trip.optimize.unknown"             // 智能优化行程时出现未知错误
	LangKeyTripOptimizePlaceInvalid       = "error.trip.optimize.place.invalid"       // 智能优化行程中的地点无效
	LangKeyTripOptimizeTransitUnsupported = "error.trip.optimize.transit.unsupported" // 当前暂不支持公交行程优化
	LangKeyTripOptimizeRouteUnavailable   = "error.trip.optimize.route.unavailable"   // 智能优化时存在不可达地点
	LangKeyTripOptimizeSourceChanged      = "error.trip.optimize.source.changed"      // 原行程在优化期间已发生变化
	LangKeyTripOptimizeStatusInvalid      = "error.trip.optimize.status.invalid"      // 当前行程状态不允许智能规划
	LangKeyTripDataCorrupt                = "error.trip.data.corrupt"                 // 行程节点数据损坏

	LangKeyTripNodeSaveUnknownErr          = "error.trip.node.save.unknown"              // 保存行程节点信息时出现未知错误
	LangKeyTripNodeDeleteUnknownErr        = "error.trip.node.delete.unknown"            // 删除行程节点信息时出现未知错误
	LangKeyTripNodeActionInvalid           = "error.trip.node.action.invalid"            // 行程节点操作类型无效
	LangKeyTripNodeIndexInvalid            = "error.trip.node.index.invalid"             // 行程节点位置无效
	LangKeyTripNodeEndpointProtected       = "error.trip.node.endpoint.protected"        // 行程起点和终点不可编辑
	LangKeyTripNodeNoteInvalid             = "error.trip.node.note.invalid"              // 行程节点备注内容无效
	LangKeyTripNodeCountInvalid            = "error.trip.node.count.invalid"             // 行程节点数量无效
	LangKeyTripNodeCompletionInvalid       = "error.trip.node.completion.invalid"        // 节点完成状态参数无效
	LangKeyTripNodeCompletionStatusInvalid = "error.trip.node.completion.status.invalid" // 当前行程状态不能设置节点完成情况
	LangKeyTripOptimizeNodeCountInvalid    = "error.trip.optimize.node.count.invalid"    // 智能优化的行程节点数量无效

	LangKeyHotPlaceRequestBodyInvalid    = "error.hot.place.request.body.invalid"  // 热门地点请求体格式无效
	LangKeyHotPlaceRequestCountInvalid   = "error.hot.place.request.count.invalid" // 热门地点请求数量无效
	LangKeyHotPlaceRequestInsufficient   = "error.hot.place.request.insufficient"  // 可用热门地点数量不足
	LangKeyHotPlaceIdentityInvalid       = "error.hot.place.identity.invalid"      // 热门地点身份格式无效
	LangKeyHotPlaceIdentityListInvalid   = "error.hot.place.identity.list.invalid" // 热门地点身份列表格式无效
	LangKeyHotPlaceImageActionInvalid    = "error.hot.place.image.action.invalid"  // 热门地点图片操作类型无效
	LangKeyHotPlaceImageRequestInvalid   = "error.hot.place.image.request.invalid" // 热门地点图片请求格式无效
	LangKeyHotPlaceCreateUnknownErr      = "error.hot.place.create.unknown"        // 创建热门地点推荐时出现未知错误
	LangKeyHotPlaceQueryNotFoundErr      = "error.hot.place.query.not.found"       // 目标热门地点推荐不存在
	LangKeyHotPlaceQueryUnknownErr       = "error.hot.place.query.unknown"         // 查询热门地点推荐时出现未知错误
	LangKeyHotPlaceUpdateNotFoundErr     = "error.hot.place.update.not.found"      // 更新热门地点推荐时未能找到该推荐数据
	LangKeyHotPlaceUpdateUnknownErr      = "error.hot.place.update.unknown"        // 更新热门地点推荐时出现未知错误
	LangKeyHotPlaceDeleteNotFoundErr     = "error.hot.place.delete.not.found"      // 删除热门地点推荐时未能找到该推荐数据
	LangKeyHotPlaceDeleteUnknownErr      = "error.hot.place.delete.unknown"        // 删除热门地点推荐时出现未知错误
	LangKeyHotPlaceImageQueryUnknownErr  = "error.hot.place.image.query.unknown"   // 查询热门地点图片时出现未知错误
	LangKeyHotPlaceImageSaveUnknownErr   = "error.hot.place.image.save.unknown"    // 保存热门地点图片时出现未知错误
	LangKeyHotPlaceImageDeleteUnknownErr = "error.hot.place.image.delete.unknown"  // 删除热门地点图片时出现未知错误
	LangKeyHotPlacePlaceInvalid          = "error.hot.place.place.invalid"         // 热门地点关联的地点无效

	LangKeyProfileRequestBodyInvalid = "error.profile.request.body.invalid" // 用户资料请求体格式无效
	LangKeyProfileActionInvalid      = "error.profile.action.invalid"       // 用户资料操作类型无效
	LangKeyProfileGenderInvalid      = "error.profile.gender.invalid"       // 用户性别无效
	LangKeyProfileAgeInvalid         = "error.profile.age.invalid"          // 用户年龄无效

	LangKeyFamilyRequestBodyInvalid      = "error.family.request.body.invalid"           // 家庭请求数据格式无效，请重试
	LangKeyFamilyIdentityInvalid         = "error.family.identity.invalid"               // 家庭信息已失效，请重新加载
	LangKeyFamilyNameInvalid             = "error.family.name.invalid"                   // 家庭名称不能为空且不能超过三十二个字
	LangKeyFamilyCreateUnknown           = "error.family.create.unknown"                 // 家庭创建失败，请稍后再试
	LangKeyFamilyUpdateUnknown           = "error.family.update.unknown"                 // 家庭名称修改失败，请稍后再试
	LangKeyFamilyQueryUnknown            = "error.family.query.unknown"                  // 家庭信息暂时无法加载，请稍后再试
	LangKeyFamilyNotFound                = "error.family.not.found"                      // 你当前还没有加入家庭
	LangKeyFamilyAlreadyJoined           = "error.family.already.joined"                 // 你已经加入了一个家庭，不能重复加入
	LangKeyFamilyInviteCodeInvalid       = "error.family.invite.code.invalid"            // 家庭邀请码无效，请重新输入
	LangKeyFamilyInviteCodeExpired       = "error.family.invite.code.expired"            // 家庭邀请码已过期，请让管理员重新生成
	LangKeyFamilyInviteCodeCreateUnknown = "error.family.invite.code.create.unknown"     // 邀请码生成失败，请稍后再试
	LangKeyFamilyJoinUnknown             = "error.family.join.unknown"                   // 加入家庭失败，请稍后再试
	LangKeyFamilyLeaveUnknown            = "error.family.leave.unknown"                  // 退出家庭失败，请稍后再试
	LangKeyFamilyMemberQueryUnknown      = "error.family.member.query.unknown"           // 家庭成员暂时无法加载，请稍后再试
	LangKeyFamilyMemberNotFound          = "error.family.member.not.found"               // 家庭成员不存在，请刷新后重试
	LangKeyFamilyMemberPermissionInvalid = "error.family.member.permission.invalid"      // 家庭成员权限无效，请重新选择
	LangKeyFamilyMemberPermissionDenied  = "error.family.member.permission.denied"       // 只有家庭管理员可以执行此操作
	LangKeyFamilyMemberPermissionSelf    = "error.family.member.permission.self.invalid" // 不能修改自己的家庭权限
	LangKeyFamilyMemberUpdateUnknown     = "error.family.member.update.unknown"          // 家庭成员更新失败，请稍后再试
	LangKeyFamilyMemberRemoveSelfInvalid = "error.family.member.remove.self.invalid"     // 不能移除管理员或当前账号
	LangKeyFamilyPinnedTripQueryUnknown  = "error.family.pinned.trip.query.unknown"      // 家庭置顶行程暂时无法加载
	LangKeyFamilyPinnedTripNotFound      = "error.family.pinned.trip.not.found"          // 该家庭置顶行程不存在
	LangKeyFamilyPinnedTripUpdateUnknown = "error.family.pinned.trip.update.unknown"     // 家庭置顶行程更新失败，请稍后再试
	LangKeyFamilyPinnedTripInvalid       = "error.family.pinned.trip.invalid"            // 只能置顶家庭成员拥有的有效行程
	LangKeyAvatarQueryActionInvalid      = "error.avatar.query.action.invalid"           // 头像查询操作类型无效
	LangKeyAvatarQueryUserInvalid        = "error.avatar.query.user.invalid"             // 头像查询目标无效
	LangKeyAvatarUploadDataInvalid       = "error.avatar.upload.data.invalid"            // 头像上传数据为空
	LangKeyAuthRequestBodyInvalid        = "error.auth.request.body.invalid"             // 认证请求体格式无效
	LangKeyMessageRequestBodyInvalid     = "error.message.request.body.invalid"          // 消息请求数据格式无效，请重试
	LangKeyMessageTypeInvalid            = "error.message.type.invalid"                  // 消息类型无效，请重新选择
	LangKeyMessageContentInvalid         = "error.message.content.invalid"               // 消息内容不能为空且不能超过两千字
	LangKeyMessageTitleInvalid           = "error.message.title.invalid"                 // 公告标题不能为空且不能超过一百二十八个字
	LangKeyMessagePermissionDenied       = "error.message.permission.denied"             // 只有家庭管理员可以发布公告
	LangKeyMessageFamilyNotFound         = "error.message.family.not.found"              // 请先加入家庭后再使用消息中心
	LangKeyMessageNotFound               = "error.message.not.found"                     // 消息不存在或已无法访问
	LangKeyMessageCreateUnknown          = "error.message.create.unknown"                // 消息发送失败，请稍后再试
	LangKeyMessageQueryUnknown           = "error.message.query.unknown"                 // 消息加载失败，请稍后再试
	LangKeyMessageReadUnknown            = "error.message.read.unknown"                  // 消息状态更新失败，请稍后再试
	LangKeySafetyRequestBodyInvalid      = "error.security.request.body.invalid"         // 安全设置请求格式无效，请重试
	LangKeySafetySettingInvalid          = "error.security.setting.invalid"              // 请至少选择一项安全设置
	LangKeySafetySettingQueryUnknown     = "error.security.setting.query.unknown"        // 安全设置暂时无法加载，请稍后再试
	LangKeySafetySettingUpdateUnknown    = "error.security.setting.update.unknown"       // 安全设置保存失败，请稍后再试
)
