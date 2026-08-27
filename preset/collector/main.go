// Run from the repository root with:
//
//	go run ./preset/collector
//
// The collector creates a curated, nationwide hot-place data pack. Images are
// downloaded from Wikimedia Commons, center-cropped to 16:9, and encoded as
// 1920x1080 JPEG files so the API can serve a predictable image shape.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Happy2018new/evercare-journey-backend/database/define"
	"github.com/Happy2018new/evercare-journey-backend/utils"
	"github.com/google/uuid"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
	_ "image/gif"
	_ "image/png"
)

const (
	imageWidth   = 1920
	imageHeight  = 1080
	imageQuality = 90
	commonsAPI   = "https://commons.wikimedia.org/w/api.php"
)

type attraction struct {
	Slug           string  `json:"slug"`
	Province       string  `json:"province"`
	City           string  `json:"city"`
	Name           string  `json:"name"`
	RecommendTitle string  `json:"recommend_title"`
	Detail         string  `json:"recommend_detail"`
	SearchQuery    string  `json:"search_query"`
	Category       string  `json:"category"`
	Longitude      float64 `json:"longitude"`
	Latitude       float64 `json:"latitude"`
}

type hotPlace struct {
	HotPlaceUniqueID uint32 `json:"HotPlaceUniqueID"`
	HotPlaceIdentity string `json:"HotPlaceIdentity"`
	RecommendTitle   string `json:"RecommendTitle"`
	RecommandDetail  string `json:"RecommandDetail"`
	PlaceImageItemID string `json:"PlaceImageItemID"`
	PlaceIdentity    string `json:"PlaceIdentity"`
}

type placeCatalogItem struct {
	PlaceIdentity   string  `json:"place_identity"`
	ProviderName    string  `json:"provider_name"`
	ProviderPlaceID string  `json:"provider_place_id"`
	Name            string  `json:"name"`
	CategoryCode    string  `json:"category_code"`
	CategoryName    string  `json:"category_name"`
	FullAddress     string  `json:"full_address"`
	Province        string  `json:"province"`
	City            string  `json:"city"`
	District        string  `json:"district"`
	AdCode          string  `json:"ad_code"`
	Longitude       float64 `json:"longitude"`
	Latitude        float64 `json:"latitude"`
}

type sourceRecord struct {
	Slug               string `json:"slug"`
	ImageFile          string `json:"image_file"`
	CommonsPage        string `json:"commons_page"`
	OriginalImageURL   string `json:"original_image_url"`
	DownloadedImageURL string `json:"downloaded_image_url"`
	License            string `json:"license"`
	Artist             string `json:"artist"`
	Attribution        string `json:"attribution"`
	SourceWidth        int    `json:"source_width"`
	SourceHeight       int    `json:"source_height"`
	OutputWidth        int    `json:"output_width"`
	OutputHeight       int    `json:"output_height"`
}

type commonsCandidate struct {
	Title          string
	PageURL        string
	ImageURL       string
	ThumbURL       string
	RedirectURL    string
	RedirectAltURL string
	ProxyURL       string
	Alternatives   []commonsCandidate `json:"-"`
	Width          int
	Height         int
	ThumbWidth     int
	ThumbHeight    int
	License        string
	Artist         string
	Credit         string
}

type commonsResponse struct {
	Query struct {
		Pages map[string]struct {
			Title     string `json:"title"`
			Canonical string `json:"canonicaltitle"`
			Info      []struct {
				URL            string `json:"url"`
				DescriptionURL string `json:"descriptionurl"`
				Width          int    `json:"width"`
				Height         int    `json:"height"`
				ThumbURL       string `json:"thumburl"`
				ThumbWidth     int    `json:"thumbwidth"`
				ThumbHeight    int    `json:"thumbheight"`
				ExtMetadata    map[string]struct {
					Value json.RawMessage `json:"value"`
				} `json:"extmetadata"`
			} `json:"imageinfo"`
		} `json:"pages"`
	} `json:"query"`
}

var attractions = []attraction{
	{Slug: "beijing-forbidden-city", Province: "北京市", City: "北京市", Name: "故宫博物院", RecommendTitle: "故宫博物院", Detail: "穿行于紫禁城的中轴线，感受明清宫殿建筑、皇家园林与文物收藏构成的历史空间。", SearchQuery: "Forbidden City Beijing", Category: "历史建筑", Longitude: 116.3972, Latitude: 39.9163},
	{Slug: "beijing-great-wall", Province: "北京市", City: "北京市", Name: "八达岭长城", RecommendTitle: "八达岭长城", Detail: "登临八达岭长城，沿着山脊远眺关山起伏，体验中国古代防御工程的雄浑尺度。", SearchQuery: "Badaling Great Wall", Category: "历史建筑", Longitude: 116.0169, Latitude: 40.3547},
	{Slug: "beijing-temple-of-heaven", Province: "北京市", City: "北京市", Name: "天坛公园", RecommendTitle: "天坛公园", Detail: "在祈年殿与圜丘坛之间了解古代礼制与建筑声学，园林空间也适合从容漫步。", SearchQuery: "Temple of Heaven Beijing", Category: "历史建筑", Longitude: 116.4108, Latitude: 39.8822},
	{Slug: "tianjin-ancient-culture-street", Province: "天津市", City: "天津市", Name: "天津古文化街", RecommendTitle: "天津古文化街", Detail: "沿海河探访津门传统街区，欣赏牌楼、民俗店铺与地方工艺，感受天津城市文化。", SearchQuery: "Tianjin Ancient Cultural Street", Category: "人文景观", Longitude: 117.1955, Latitude: 39.1532},
	{Slug: "hebei-chengde-mountain-resort", Province: "河北省", City: "承德市", Name: "承德避暑山庄", RecommendTitle: "承德避暑山庄", Detail: "在山水园林与宫殿区之间感受清代皇家园林的复合格局，远眺外八庙建筑群。", SearchQuery: "Chengde Mountain Resort", Category: "历史建筑", Longitude: 117.9388, Latitude: 40.9917},
	{Slug: "hebei-shanhaiguan", Province: "河北省", City: "秦皇岛市", Name: "山海关", RecommendTitle: "山海关", Detail: "从天下第一关到老龙头，观察长城连接山海的关隘体系与滨海景观。", SearchQuery: "Shanhaiguan Great Wall", Category: "历史建筑", Longitude: 119.7756, Latitude: 40.0028},
	{Slug: "shanxi-yungang-grottoes", Province: "山西省", City: "大同市", Name: "云冈石窟", RecommendTitle: "云冈石窟", Detail: "近距离欣赏北魏石窟造像与壁刻艺术，理解佛教艺术在不同文化之间的交流。", SearchQuery: "Yungang Grottoes", Category: "世界遗产", Longitude: 113.1326, Latitude: 40.1109},
	{Slug: "shanxi-pingyao-ancient-city", Province: "山西省", City: "晋中市", Name: "平遥古城", RecommendTitle: "平遥古城", Detail: "漫步保存完整的明清县城格局，串联城墙、票号、县衙与传统民居。", SearchQuery: "Pingyao Ancient City", Category: "世界遗产", Longitude: 112.1760, Latitude: 37.1894},
	{Slug: "inner-mongolia-xiangshawan", Province: "内蒙古自治区", City: "鄂尔多斯市", Name: "响沙湾", RecommendTitle: "响沙湾", Detail: "在沙丘、沙漠绿洲与开阔天际线之间体验西北沙漠景观，适合安排轻量户外活动。", SearchQuery: "Xiangshawan desert", Category: "自然景观", Longitude: 109.9900, Latitude: 40.2860},
	{Slug: "inner-mongolia-hulunbuir", Province: "内蒙古自治区", City: "呼伦贝尔市", Name: "呼伦贝尔草原", RecommendTitle: "呼伦贝尔草原", Detail: "沿草原河流与牧场公路远行，观察季节性草原景观、牧业文化与辽阔边地风光。", SearchQuery: "Hulunbuir Grassland", Category: "自然景观", Longitude: 119.7658, Latitude: 49.2116},
	{Slug: "liaoning-shenyang-imperial-palace", Province: "辽宁省", City: "沈阳市", Name: "沈阳故宫", RecommendTitle: "沈阳故宫", Detail: "在盛京宫殿群中了解清朝入关前的宫廷建筑与民族文化融合。", SearchQuery: "Mukden Palace Shenyang", Category: "历史建筑", Longitude: 123.4536, Latitude: 41.7964},
	{Slug: "liaoning-dalian-xinghai-square", Province: "辽宁省", City: "大连市", Name: "大连星海广场", RecommendTitle: "大连星海广场", Detail: "沿滨海广场与城市天际线散步，感受大连开阔的海湾城市景观。", SearchQuery: "Xinghai Square Dalian", Category: "城市景观", Longitude: 121.5878, Latitude: 38.8667},
	{Slug: "jilin-changbai-mountain", Province: "吉林省", City: "延边州", Name: "长白山", RecommendTitle: "长白山", Detail: "从火山地貌、天池到原始森林，体验东北高山生态系统随海拔变化的层次。", SearchQuery: "Changbai Mountain Tianchi", Category: "自然景观", Longitude: 128.0765, Latitude: 42.0060},
	{Slug: "heilongjiang-saint-sophia", Province: "黑龙江省", City: "哈尔滨市", Name: "圣索菲亚教堂", RecommendTitle: "哈尔滨圣索菲亚教堂", Detail: "欣赏哈尔滨代表性的拜占庭式建筑，在中央大街周边感受多元城市历史。", SearchQuery: "Saint Sophia Cathedral Harbin", Category: "历史建筑", Longitude: 126.6229, Latitude: 45.7650},
	{Slug: "shanghai-bund", Province: "上海市", City: "上海市", Name: "外滩", RecommendTitle: "上海外滩", Detail: "沿黄浦江观察外滩历史建筑群与陆家嘴天际线，感受上海的近代城市脉络。", SearchQuery: "The Bund Shanghai", Category: "城市景观", Longitude: 121.4903, Latitude: 31.2400},
	{Slug: "shanghai-yu-garden", Province: "上海市", City: "上海市", Name: "豫园", RecommendTitle: "豫园", Detail: "在曲折园路、叠山理水与江南建筑之间，体验老城厢园林的精巧尺度。", SearchQuery: "Yu Garden Shanghai", Category: "园林", Longitude: 121.4920, Latitude: 31.2271},
	{Slug: "jiangsu-humble-administrators-garden", Province: "江苏省", City: "苏州市", Name: "拙政园", RecommendTitle: "苏州拙政园", Detail: "沿水院、借景与花窗游览江南古典园林，观察建筑与自然景观的层层展开。", SearchQuery: "Humble Administrator's Garden Suzhou", Category: "园林", Longitude: 120.6298, Latitude: 31.3246},
	{Slug: "jiangsu-sun-yat-sen-mausoleum", Province: "江苏省", City: "南京市", Name: "中山陵", RecommendTitle: "南京中山陵", Detail: "沿长阶登临中山陵，感受钟山山林、轴线建筑与近代历史共同形成的景观。", SearchQuery: "Sun Yat-sen Mausoleum Nanjing", Category: "历史建筑", Longitude: 118.8488, Latitude: 32.0594},
	{Slug: "zhejiang-west-lake", Province: "浙江省", City: "杭州市", Name: "西湖", RecommendTitle: "杭州西湖", Detail: "沿湖岸串联苏堤、断桥与湖山景色，体验自然山水与城市公共空间的融合。", SearchQuery: "West Lake Hangzhou", Category: "世界遗产", Longitude: 120.1480, Latitude: 30.2431},
	{Slug: "zhejiang-mount-putuo", Province: "浙江省", City: "舟山市", Name: "普陀山", RecommendTitle: "普陀山", Detail: "在海岛山林与佛教寺院之间缓行，感受海天景观、宗教文化与滨海生态。", SearchQuery: "Mount Putuo", Category: "宗教文化", Longitude: 122.3880, Latitude: 30.0002},
	{Slug: "anhui-huangshan", Province: "安徽省", City: "黄山市", Name: "黄山", RecommendTitle: "黄山", Detail: "欣赏奇松、怪石、云海与山峰形成的经典山岳景观，合理安排登山线路与休息节奏。", SearchQuery: "Huangshan Mountain", Category: "世界遗产", Longitude: 118.1728, Latitude: 30.1310},
	{Slug: "anhui-hongcun", Province: "安徽省", City: "黄山市", Name: "宏村", RecommendTitle: "宏村", Detail: "沿月沼、水圳与徽派民居游览，观察古村落水系、建筑与田园环境的整体规划。", SearchQuery: "Hongcun village Anhui", Category: "世界遗产", Longitude: 117.9993, Latitude: 30.0117},
	{Slug: "fujian-gulangyu", Province: "福建省", City: "厦门市", Name: "鼓浪屿", RecommendTitle: "厦门鼓浪屿", Detail: "漫步岛上历史街区、海岸与花园洋房，感受闽南海岛生活与近代建筑遗存。", SearchQuery: "Gulangyu Island Xiamen", Category: "世界遗产", Longitude: 118.0660, Latitude: 24.4488},
	{Slug: "fujian-wuyi-mountains", Province: "福建省", City: "南平市", Name: "武夷山", RecommendTitle: "武夷山", Detail: "乘竹筏或沿山径观察丹霞峰林、九曲溪与茶文化景观，体验闽北山水。", SearchQuery: "Wuyi Mountains Fujian", Category: "世界遗产", Longitude: 117.7170, Latitude: 27.7560},
	{Slug: "jiangxi-lushan", Province: "江西省", City: "九江市", Name: "庐山", RecommendTitle: "庐山", Detail: "在山谷、瀑布与云雾间探索庐山景观，串联自然风光与近代建筑群。", SearchQuery: "Mount Lu Lushan", Category: "世界遗产", Longitude: 115.9920, Latitude: 29.5600},
	{Slug: "jiangxi-sanqing-mountain", Province: "江西省", City: "上饶市", Name: "三清山", RecommendTitle: "三清山", Detail: "沿高山栈道观赏花岗岩峰林、奇松与云海，感受道教名山的自然与人文气质。", SearchQuery: "Sanqing Mountain", Category: "自然景观", Longitude: 118.0630, Latitude: 28.9020},
	{Slug: "shandong-mount-tai", Province: "山东省", City: "泰安市", Name: "泰山", RecommendTitle: "泰山", Detail: "沿传统登山路线穿行山门、石刻与峰顶，体验五岳之首的山岳文化与日出景观。", SearchQuery: "Mount Tai", Category: "世界遗产", Longitude: 117.1030, Latitude: 36.2500},
	{Slug: "shandong-qufu-confucius-temple", Province: "山东省", City: "曲阜市", Name: "曲阜三孔", RecommendTitle: "曲阜三孔", Detail: "参观孔庙、孔府与孔林，了解儒家文化遗产与古代礼制建筑的延续。", SearchQuery: "Temple of Confucius Qufu", Category: "世界遗产", Longitude: 116.9890, Latitude: 35.5960},
	{Slug: "henan-longmen-grottoes", Province: "河南省", City: "洛阳市", Name: "龙门石窟", RecommendTitle: "洛阳龙门石窟", Detail: "沿伊河两岸欣赏北魏至唐代石窟造像，感受中国石刻艺术的时代变化。", SearchQuery: "Longmen Grottoes Luoyang", Category: "世界遗产", Longitude: 112.4690, Latitude: 34.5580},
	{Slug: "henan-shaolin-temple", Province: "河南省", City: "登封市", Name: "少林寺", RecommendTitle: "嵩山少林寺", Detail: "在少林寺、塔林与嵩山山麓之间了解禅宗文化、武术传统与历史建筑。", SearchQuery: "Shaolin Temple", Category: "宗教文化", Longitude: 112.9388, Latitude: 34.5096},
	{Slug: "hubei-yellow-crane-tower", Province: "湖北省", City: "武汉市", Name: "黄鹤楼", RecommendTitle: "武汉黄鹤楼", Detail: "登楼俯瞰长江与武汉城市景观，结合诗词与建筑历史理解江城地标的文化意象。", SearchQuery: "Yellow Crane Tower Wuhan", Category: "历史建筑", Longitude: 114.3075, Latitude: 30.5447},
	{Slug: "hubei-wudang-mountains", Province: "湖北省", City: "十堰市", Name: "武当山", RecommendTitle: "武当山", Detail: "沿古建筑群与山地步道探索道教名山，欣赏宫观、峰林与云雾相互映衬的景色。", SearchQuery: "Wudang Mountains", Category: "世界遗产", Longitude: 110.9960, Latitude: 32.4000},
	{Slug: "hunan-zhangjiajie", Province: "湖南省", City: "张家界市", Name: "张家界国家森林公园", RecommendTitle: "张家界国家森林公园", Detail: "在石英砂岩峰林、峡谷与森林之间游览，感受电影般的立体山水空间。", SearchQuery: "Zhangjiajie National Forest Park", Category: "自然景观", Longitude: 110.4790, Latitude: 29.3250},
	{Slug: "hunan-fenghuang-ancient-town", Province: "湖南省", City: "湘西州", Name: "凤凰古城", RecommendTitle: "凤凰古城", Detail: "沿沱江两岸欣赏吊脚楼、古桥与夜色街巷，体验湘西历史城镇的水岸风貌。", SearchQuery: "Fenghuang Ancient Town Hunan", Category: "历史建筑", Longitude: 109.5996, Latitude: 27.9480},
	{Slug: "hunan-yueyang-tower", Province: "湖南省", City: "岳阳市", Name: "岳阳楼", RecommendTitle: "岳阳楼", Detail: "登临洞庭湖畔名楼，结合湖景、楼阁与古代文学理解岳阳楼的文化价值。", SearchQuery: "Yueyang Tower", Category: "历史建筑", Longitude: 113.1150, Latitude: 29.3830},
	{Slug: "guangdong-danxia-mountain", Province: "广东省", City: "韶关市", Name: "丹霞山", RecommendTitle: "丹霞山", Detail: "观察典型丹霞地貌的赤壁、峰林与河谷，安排轻徒步即可获得丰富的地质景观。", SearchQuery: "Danxia Mountain Guangdong", Category: "世界遗产", Longitude: 113.7430, Latitude: 25.3710},
	{Slug: "guangdong-kaiping-diaolou", Province: "广东省", City: "江门市", Name: "开平碉楼与村落", RecommendTitle: "开平碉楼与村落", Detail: "在乡村田园中寻找中西合璧的碉楼建筑，了解侨乡历史与地方聚落的独特风貌。", SearchQuery: "Kaiping Diaolou", Category: "世界遗产", Longitude: 112.6980, Latitude: 22.3760},
	{Slug: "guangxi-li-river", Province: "广西壮族自治区", City: "桂林市", Name: "漓江", RecommendTitle: "桂林漓江", Detail: "沿漓江欣赏喀斯特峰丛、江面倒影与田园村落，感受桂林山水的经典构图。", SearchQuery: "Li River Guilin", Category: "自然景观", Longitude: 110.4950, Latitude: 25.2730},
	{Slug: "guangxi-longji-rice-terraces", Province: "广西壮族自治区", City: "桂林市", Name: "龙脊梯田", RecommendTitle: "龙脊梯田", Detail: "顺着山地梯田与村寨步道游览，观察不同季节水田、山林与少数民族村落的层次。", SearchQuery: "Longji Rice Terraces", Category: "自然景观", Longitude: 110.1540, Latitude: 25.8160},
	{Slug: "hainan-wuzhizhou-island", Province: "海南省", City: "三亚市", Name: "蜈支洲岛", RecommendTitle: "蜈支洲岛", Detail: "在清澈海湾、珊瑚礁与热带植被之间游览，体验海南岛的滨海自然景观。", SearchQuery: "Wuzhizhou Island Sanya", Category: "自然景观", Longitude: 109.7590, Latitude: 18.3120},
	{Slug: "hainan-yalong-bay", Province: "海南省", City: "三亚市", Name: "亚龙湾", RecommendTitle: "三亚亚龙湾", Detail: "沿海湾沙滩与热带林带散步，安排海滨休闲与自然观察，感受海南的热带海岸线。", SearchQuery: "Yalong Bay Sanya", Category: "自然景观", Longitude: 109.6530, Latitude: 18.2290},
	{Slug: "chongqing-wulong-karst", Province: "重庆市", City: "重庆市", Name: "武隆喀斯特", RecommendTitle: "武隆喀斯特", Detail: "穿行天生三桥、龙水峡地缝等喀斯特地貌，感受峡谷、天坑与桥洞的空间变化。", SearchQuery: "Wulong Karst Chongqing", Category: "世界遗产", Longitude: 107.7590, Latitude: 29.3250},
	{Slug: "chongqing-hongya-cave", Province: "重庆市", City: "重庆市", Name: "洪崖洞", RecommendTitle: "重庆洪崖洞", Detail: "从嘉陵江与长江交汇处观察山城立体建筑，夜间可欣赏传统吊脚楼风格的城市景观。", SearchQuery: "Hongya Cave Chongqing", Category: "城市景观", Longitude: 106.5860, Latitude: 29.5630},
	{Slug: "sichuan-jiuzhaigou", Province: "四川省", City: "阿坝州", Name: "九寨沟", RecommendTitle: "九寨沟", Detail: "在彩林、雪山、瀑布与钙华湖泊之间游览，感受高原河谷生态景观的清澈层次。", SearchQuery: "Jiuzhaigou National Park", Category: "世界遗产", Longitude: 103.9180, Latitude: 33.2600},
	{Slug: "sichuan-mount-emei", Province: "四川省", City: "乐山市", Name: "峨眉山", RecommendTitle: "峨眉山", Detail: "沿山地森林与寺院古道登临，观察从亚热带到高山生态的垂直变化。", SearchQuery: "Mount Emei", Category: "世界遗产", Longitude: 103.3380, Latitude: 29.5250},
	{Slug: "sichuan-leshan-buddha", Province: "四川省", City: "乐山市", Name: "乐山大佛", RecommendTitle: "乐山大佛", Detail: "在三江汇流处观赏依山开凿的大佛与凌云山景观，理解唐代石刻工程的规模。", SearchQuery: "Leshan Giant Buddha", Category: "世界遗产", Longitude: 103.7730, Latitude: 29.5440},
	{Slug: "guizhou-huangguoshu-waterfall", Province: "贵州省", City: "安顺市", Name: "黄果树瀑布", RecommendTitle: "黄果树瀑布", Detail: "沿水帘洞、瀑布与峡谷步道游览，感受喀斯特地貌中水流塑造的立体景观。", SearchQuery: "Huangguoshu Waterfall", Category: "自然景观", Longitude: 105.6720, Latitude: 25.9910},
	{Slug: "guizhou-xijiang-miao-village", Province: "贵州省", City: "黔东南州", Name: "西江千户苗寨", RecommendTitle: "西江千户苗寨", Detail: "在山谷村寨、风雨桥与梯田之间了解苗族聚落形态与传统生活方式。", SearchQuery: "Xijiang Qianhu Miao Village", Category: "人文景观", Longitude: 108.1760, Latitude: 26.5050},
	{Slug: "yunnan-stone-forest", Province: "云南省", City: "昆明市", Name: "石林", RecommendTitle: "昆明石林", Detail: "在喀斯特石峰、石柱与步道之间观察岩溶地貌，感受彝族撒尼文化的地方表达。", SearchQuery: "Stone Forest Kunming", Category: "世界遗产", Longitude: 103.3240, Latitude: 24.8110},
	{Slug: "yunnan-lijiang-old-town", Province: "云南省", City: "丽江市", Name: "丽江古城", RecommendTitle: "丽江古城", Detail: "沿水系、石板街与纳西族传统建筑游览，体验高原古城的生活尺度与文化肌理。", SearchQuery: "Lijiang Old Town", Category: "世界遗产", Longitude: 100.2340, Latitude: 26.8720},
	{Slug: "yunnan-dali-ancient-town", Province: "云南省", City: "大理州", Name: "大理古城", RecommendTitle: "大理古城", Detail: "在苍山洱海之间漫步古城街巷，感受白族建筑、地方市集与高原湖山景观。", SearchQuery: "Dali Ancient Town Yunnan", Category: "历史建筑", Longitude: 100.1680, Latitude: 25.6940},
	{Slug: "tibet-potala-palace", Province: "西藏自治区", City: "拉萨市", Name: "布达拉宫", RecommendTitle: "布达拉宫", Detail: "从拉萨城市视角欣赏依山而建的宫殿群，了解藏传佛教建筑与高原城市文化。", SearchQuery: "Potala Palace Lhasa", Category: "世界遗产", Longitude: 91.1172, Latitude: 29.6550},
	{Slug: "tibet-namtso", Province: "西藏自治区", City: "拉萨市", Name: "纳木错", RecommendTitle: "纳木错", Detail: "在高原湖泊、雪山与开阔草原之间感受西藏自然景观的尺度与光线变化。", SearchQuery: "Namtso Lake Tibet", Category: "自然景观", Longitude: 90.6000, Latitude: 30.7000},
	{Slug: "shaanxi-terracotta-army", Province: "陕西省", City: "西安市", Name: "秦始皇帝陵博物院", RecommendTitle: "秦始皇兵马俑", Detail: "在一号坑等展厅观察秦代军阵与陶俑细节，理解秦始皇帝陵的考古价值。", SearchQuery: "Terracotta Army Xi'an", Category: "世界遗产", Longitude: 109.2780, Latitude: 34.3850},
	{Slug: "shaanxi-mount-hua", Province: "陕西省", City: "渭南市", Name: "华山", RecommendTitle: "华山", Detail: "沿险峻山脊、峰顶与栈道体验华山地貌，登山前应根据天气与体力安排线路。", SearchQuery: "Mount Hua Huashan", Category: "自然景观", Longitude: 110.0870, Latitude: 34.5450},
	{Slug: "shaanxi-xian-city-wall", Province: "陕西省", City: "西安市", Name: "西安城墙", RecommendTitle: "西安城墙", Detail: "在完整的古城墙体系上骑行或步行，观察城门、角楼与现代城市的并置关系。", SearchQuery: "Xi'an City Wall", Category: "历史建筑", Longitude: 108.9470, Latitude: 34.2580},
	{Slug: "gansu-mogao-caves", Province: "甘肃省", City: "敦煌市", Name: "莫高窟", RecommendTitle: "敦煌莫高窟", Detail: "通过预约参观洞窟与数字展示，了解敦煌壁画、彩塑及丝绸之路文化交流。", SearchQuery: "Mogao Caves Dunhuang", Category: "世界遗产", Longitude: 94.6830, Latitude: 40.0400},
	{Slug: "gansu-zhangye-danxia", Province: "甘肃省", City: "张掖市", Name: "张掖七彩丹霞", RecommendTitle: "张掖七彩丹霞", Detail: "在观景台欣赏彩色丘陵与峡谷地貌，适合在日出或日落时观察地层色彩变化。", SearchQuery: "Zhangye Danxia", Category: "自然景观", Longitude: 100.1160, Latitude: 38.9190},
	{Slug: "qinghai-qinghai-lake", Province: "青海省", City: "海南州", Name: "青海湖", RecommendTitle: "青海湖", Detail: "沿湖岸远眺高原湖面与雪山草原，体验青海湖随季节变化的辽阔生态景观。", SearchQuery: "Qinghai Lake", Category: "自然景观", Longitude: 100.1900, Latitude: 36.8900},
	{Slug: "qinghai-chaka-salt-lake", Province: "青海省", City: "海西州", Name: "茶卡盐湖", RecommendTitle: "茶卡盐湖", Detail: "在盐湖镜面、盐晶与高原天际线之间观察独特地貌，注意保护脆弱的盐湖环境。", SearchQuery: "Chaka Salt Lake", Category: "自然景观", Longitude: 99.0800, Latitude: 36.7900},
	{Slug: "ningxia-shapotou", Province: "宁夏回族自治区", City: "中卫市", Name: "沙坡头", RecommendTitle: "中卫沙坡头", Detail: "在黄河、沙漠与绿洲交汇处观察荒漠治理成果，感受宁夏独特的河沙景观。", SearchQuery: "Shapotou Ningxia", Category: "自然景观", Longitude: 105.0010, Latitude: 37.4780},
	{Slug: "ningxia-western-xia-tombs", Province: "宁夏回族自治区", City: "银川市", Name: "西夏王陵", RecommendTitle: "西夏王陵", Detail: "在贺兰山麓的陵墓遗址中了解西夏历史，观察夯土建筑与荒漠地貌的结合。", SearchQuery: "Western Xia Tombs", Category: "历史遗址", Longitude: 105.9750, Latitude: 38.6180},
	{Slug: "xinjiang-kanas", Province: "新疆维吾尔自治区", City: "阿勒泰地区", Name: "喀纳斯", RecommendTitle: "喀纳斯", Detail: "在高山湖泊、针叶林与草原河谷之间游览，感受新疆北部四季分明的自然景观。", SearchQuery: "Kanas Lake Xinjiang", Category: "自然景观", Longitude: 87.0140, Latitude: 48.8200},
	{Slug: "xinjiang-tianchi", Province: "新疆维吾尔自治区", City: "昌吉州", Name: "天山天池", RecommendTitle: "天山天池", Detail: "从湖岸远眺博格达峰与高山森林，体验天山冰川地貌孕育的湖泊景观。", SearchQuery: "Tianchi Lake Xinjiang", Category: "自然景观", Longitude: 88.1260, Latitude: 43.8830},
	{Slug: "xinjiang-kashgar-old-city", Province: "新疆维吾尔自治区", City: "喀什市", Name: "喀什古城", RecommendTitle: "喀什古城", Detail: "沿传统街巷、民居与市集游览，感受丝绸之路节点城市的多民族生活与建筑风貌。", SearchQuery: "Kashgar Old City", Category: "人文景观", Longitude: 75.9890, Latitude: 39.4700},
}

func main() {
	outDir := flag.String("out", "preset", "output directory")
	validateOnly := flag.Bool("validate", false, "validate an existing output directory without network access")
	enrichOnly := flag.Bool("enrich", false, "refresh descriptions in an existing output directory without network access")
	categoriesOnly := flag.Bool("categories", false, "refresh curated category names without network access")
	amapOnly := flag.Bool("amap", false, "refresh place_catalog.json from Amap POI data")
	flag.Parse()

	if *validateOnly {
		if err := validate(*outDir); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("validated hot-place pack in %s\n", *outDir)
		return
	}
	if *enrichOnly {
		if err := enrichExisting(*outDir); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("enriched hot-place descriptions in %s\n", *outDir)
		return
	}
	if *categoriesOnly {
		if err := refreshCategories(*outDir); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("refreshed %d curated category names in %s\n", len(attractions), *outDir)
		return
	}
	if *amapOnly {
		if err := syncAmapPlaces(*outDir); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("refreshed %d Amap POIs in %s\n", len(attractions), *outDir)
		return
	}
	if err := collect(*outDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var detailedNotes = map[string]string{
	"beijing-forbidden-city":               "故宫以中轴线为骨架展开，午门、太和门、三大殿、后三宫和御花园依次形成层次分明的宫城空间。游览时可以留意建筑等级、色彩、屋顶形式和院落尺度，也可以结合展厅了解书画、陶瓷、宫廷生活用品等文物。不同展馆的开放安排可能变化，适合提前规划入口、展览和休息节点，避免只在中心建筑前短暂停留。",
	"beijing-great-wall":                   "八达岭段长城依山势修筑，城墙、敌楼、烽火台和山脊线共同构成连续的防御景观。登城过程中可以观察墙体随坡度变化的台阶和垛口，也能从不同敌楼获得关沟、群山和长城走势的视角。部分路段坡度较大，建议根据体力选择往返方向，穿着防滑鞋，并为排队、登高和天气变化预留时间。",
	"beijing-temple-of-heaven":             "天坛的祈年殿、皇穹宇、圜丘坛和回音壁集中体现了古代祭祀建筑的轴线、比例和声学设计。除了重点建筑，还可以观察古柏、丹陛桥、长廊与开阔坛域如何共同营造庄重而有秩序的空间。适合沿中轴线慢慢游览，再进入园林区域感受城市公园的日常使用，拍摄时可利用清晨或傍晚的柔和光线表现建筑轮廓。",
	"tianjin-ancient-culture-street":       "天津古文化街沿海河展开，牌楼、传统店铺、民俗工艺和地方饮食共同构成具有津味的历史街区。游览重点不仅是街面建筑，也包括店铺中的年画、泥人、风筝和曲艺等文化线索。可以把街区与海河岸线、周边老城建筑安排在同一条步行线路中，白天适合看细节和买手工艺品，傍晚则更适合观察灯光、河景与人流形成的城市氛围。",
	"hebei-chengde-mountain-resort":        "承德避暑山庄将宫殿、湖区、平原和山地组合在一座大型皇家园林中，空间变化比单一宫殿景点更丰富。宫墙以内可以观察清代宫廷生活与礼制建筑，湖区适合从水面、桥梁和岛屿角度理解园林借景，山地则展现北方自然地貌与皇家园林经营之间的关系。游览时建议按区域分段，不必追求一次走完全部山路，并结合远处外八庙的城市景观理解承德整体格局。",
	"hebei-shanhaiguan":                    "山海关的价值在于长城、关城、山地和海岸在同一处发生联系。天下第一关展示关隘城防与交通节点的历史功能，老龙头则让长城伸入海中的形态成为直观的滨海景观。游览时可以从城门、城墙和敌楼观察防御体系，再到海边感受地形尺度和风向变化；夏季日照较强，海边步行应准备饮水、防晒用品，并为不同区域之间的交通预留时间。",
	"shanxi-yungang-grottoes":              "云冈石窟依山开凿，现存洞窟、佛像、浮雕和壁面装饰共同记录了北魏时期的宗教艺术与文化交流。不同洞窟在造像尺度、衣纹表现、建筑式样和色彩残留方面各有特点，适合在参观前先了解石窟编号和保护规则，再把注意力放到局部细节而不只是大型佛像。洞窟内部光线较弱且保护要求严格，应遵守禁止拍摄或限制停留的规定，按照开放路线安静参观。",
	"shanxi-pingyao-ancient-city":          "平遥古城保留了较完整的城墙、街巷、县衙、票号、镖局和传统民居体系，适合以步行方式理解中国传统县城的空间结构。登城墙可以看到街区棋盘式布局，日升昌等票号遗存则展示晋商金融活动，县衙和民居能够补充对地方治理与生活尺度的认识。建议避开只在主街购物的路线，把城墙、支巷、院落和夜间街景结合起来，进入院落时尊重现存建筑和居民生活。",
	"inner-mongolia-xiangshawan":           "响沙湾以连续沙丘、沙漠边缘植被和开阔天际线构成典型的荒漠景观，风力和光线变化会让沙脊纹理呈现不同层次。游览可以围绕沙丘远眺、沙漠步行、绿洲景观和地方文化体验展开，重点观察沙漠并非单一色块，而是由坡面、阴影、风痕和稀疏植被组成的动态地貌。沙地行走消耗体力较大，应根据天气安排活动，保护眼睛和皮肤，并避免踩踏脆弱植被。",
	"inner-mongolia-hulunbuir":             "呼伦贝尔草原的魅力来自草原、河流、湿地、森林和牧场共同形成的辽阔生态空间，不同季节会呈现完全不同的色彩和天气。沿公路或河谷行进时，可以观察草场起伏、牧业聚落、蒙古族生活方式和边地城市之间的关系。适合把长距离移动本身作为行程的一部分，在观景台、村落和河岸安排短暂停留；草原天气变化快，注意保暖、防晒和保护草场，不随意驶入或进入封闭区域。",
	"liaoning-shenyang-imperial-palace":    "沈阳故宫记录了清朝入关前的宫廷制度与建筑发展，宫殿布局、满族生活线索以及汉地宫殿形式的吸收都具有较强的观察价值。大政殿和十王亭等建筑体现早期宫廷与政治空间，后部宫殿则展现日常居住和礼仪功能。游览时可以按照前朝、内廷和附属展馆分区理解，不仅看单座建筑的外观，也留意院落之间的交通关系、装饰细节和展陈文物。",
	"liaoning-dalian-xinghai-square":       "星海广场以开阔的城市公共空间、海湾视野和滨海步道形成大连的代表性城市景观。广场尺度适合观察城市轴线、雕塑、绿地和海面之间的关系，沿海岸继续步行则可以获得更连续的海湾视角。白天适合看建筑和地平线，傍晚适合拍摄城市灯光与海面变化。滨海区域风力较大，建议根据天气安排停留时间，并把周边海岸线、城市建筑和公共空间作为整体来体验。",
	"jilin-changbai-mountain":              "长白山以火山地貌为基础，天池、火山口、瀑布、岳桦林和针叶林构成随海拔变化的生态梯度。天池视野受天气影响明显，山地云雾、风雪和光线变化会不断改变景观层次；沿途森林和溪流则能补充对火山山区生态环境的认识。不同季节和景区开放安排差异较大，应根据官方信息选择路线，注意高海拔天气、保暖和步道安全，不在非开放区域停留。",
	"heilongjiang-saint-sophia":            "圣索菲亚教堂以红砖墙体、绿色穹顶和拜占庭式体量成为哈尔滨城市历史的鲜明标志，周边广场与近代街区共同构成多元建筑文化的展示窗口。除了正立面和穹顶，还可以观察砖石细节、拱券、窗洞和建筑与广场的尺度关系。建议把教堂与中央大街、老建筑群安排为步行线路，白天看建筑结构，夜间看灯光和人流变化，同时关注室内展陈或临时开放信息。",
	"shanghai-bund":                        "外滩沿黄浦江排列的历史建筑群集中呈现上海近代金融、航运和城市建设的痕迹，对岸陆家嘴的高层天际线则构成鲜明的时代对照。可以沿江观察建筑立面、塔楼、穹顶和不同风格之间的连续界面，也可以从不同时间段比较雾气、日落和夜景对城市轮廓的影响。建议安排一段完整的滨江步行，不只在观景平台停留，并注意节假日人流、江风和拍摄时的安全距离。",
	"shanghai-yu-garden":                   "豫园以水池、假山、曲廊、花窗、亭台和院落组织江南古典园林的游览节奏，空间并不追求一眼望尽，而是通过转折和借景不断改变视线。游览时可以观察太湖石叠山、漏窗构图、临水建筑和植物季相，也可以结合周边城隍庙片区理解老城厢的商业与民俗延续。园内道路和空间较紧凑，建议放慢速度，注意保护古建筑和园林构件，避开人流高峰以获得更完整的空间感。",
	"jiangsu-humble-administrators-garden": "拙政园以水面为中心，通过远香堂、荷风四面亭、曲桥、岛屿和廊道组织出开合相间的园林景观。借景、框景和对景是游览时值得重点观察的设计方法，同一座亭台从不同角度看会呈现不同的水面、植物和建筑关系。春夏适合看植物与水景，秋冬更容易看清园林骨架和建筑细节。建议预留较充足时间慢行，不把参观压缩为拍照打卡，并留意园方对部分区域的单向路线安排。",
	"jiangsu-sun-yat-sen-mausoleum":        "中山陵依钟山山势布局，牌坊、墓道、石阶、陵门和祭堂共同形成具有强烈轴线感的近代纪念建筑群。登阶过程中可以感受地形、建筑和视线如何逐层收束，抵达高处后再回望南京城市和山林景观。陵区周边还有钟山的森林、道路和历史遗存，适合与明孝陵或灵谷寺等区域组合游览。建议穿舒适鞋，按体力分配登阶节奏，并尊重纪念场所的安静氛围。",
	"zhejiang-west-lake":                   "西湖由湖面、苏堤、白堤、孤山、山林、古寺和城市岸线共同组成，景观价值不仅在于某一处水面，也在于自然山水与杭州日常生活的长期融合。沿湖步行或骑行可以观察桥、堤、柳岸、远山和建筑在不同视角下的组合，天气和季节会显著改变湖山层次。建议把核心景点与较安静的湖岸、博物馆或山林步道结合，给行程留出停留时间，节假日注意人流和骑行安全。",
	"zhejiang-mount-putuo":                 "普陀山将海岛山林、寺院建筑、海岸礁石和观音文化结合在一起，普济寺、法雨寺、慧济寺等区域体现不同的宗教空间和山地环境。步行或乘接驳车移动时，可以观察寺院与古树、坡地、海风之间的关系；沿海岸线路则能看到岛屿地貌和开阔海景。游览应保持安静、尊重宗教仪式和建筑秩序，山地路段注意防滑，提前了解船班、接驳和各区域开放时间。",
	"anhui-huangshan":                      "黄山以花岗岩峰林为骨架，奇松、怪石、云海、温泉和冬雪共同构成层次丰富的山岳景观。不同高度和朝向会带来完全不同的视线，迎客松、光明顶、西海大峡谷等区域各有代表性，但实际开放情况会受天气和地质条件影响。登山时建议根据体力选择线路，给上下山、观景和休息预留足够时间；山路湿滑或有风雪时应服从管理，不在危险边缘停留，也要带走个人垃圾。",
	"anhui-hongcun":                        "宏村以月沼、南湖、水圳和徽派民居组成的村落水系著称，民居、祠堂、巷道与田园环境之间形成完整的传统聚落景观。沿水圳行走可以理解古村落如何利用地形和水资源组织生活，进入承志堂等建筑则能观察木雕、砖雕、石雕和天井空间。清晨和傍晚的光线适合表现白墙黛瓦与水面倒影，游览时请尊重居民生活，不随意进入私人区域，并保护古建筑和水系环境。",
	"fujian-gulangyu":                      "鼓浪屿以海岛环境、近代建筑、窄巷、花园和音乐文化构成独特的步行型历史城区。不同风格的别墅、教堂、领事机构旧址和民居分散在岛上，适合通过街巷漫步观察建筑细节，而不是只停留在一个观景点。海岸、坡道和植物景观会不断改变视线，建议穿舒适鞋并为迷路和临时停留留出时间。轮渡安排、岛上客流和部分建筑开放情况会变化，应提前确认并维护安静的社区环境。",
	"fujian-wuyi-mountains":                "武夷山把丹霞峰林、九曲溪、森林、茶园和朱子文化等人文线索结合在同一片山水空间中。乘竹筏可以从水面连续观察山体、崖壁、倒影和沿岸植被，步道则能进入更细致的溪谷和茶文化景观。不同天气下山水色彩变化明显，适合安排水上游览与短途步行相结合。山地和水上活动都应服从调度，注意防晒、防滑和个人物品固定，不攀爬未开放岩壁。",
	"jiangxi-lushan":                       "庐山以云雾山景、瀑布、峡谷、森林、湖泊和近代建筑群形成复合型山地景观，牯岭镇还保留了独特的山地城市生活尺度。三叠泉、含鄱口、五老峰和庐山会议旧址等区域分别体现自然景观和近代历史的不同侧面。游览适合分成山镇、峰顶和瀑布线路，不要只依赖车辆快速串联。山中天气多变，注意雾天能见度、台阶湿滑和体力安排，按照开放路线行走。",
	"jiangxi-sanqing-mountain":             "三清山的花岗岩峰林、峰柱、奇松和云海在高山栈道上呈现出连续变化，玉京峰、阳光海岸和西海岸等区域各有不同的观景方向。栈道让游客能够从侧面、俯视和远眺多个角度理解山体形态，也能看到道教文化与自然山岳结合的痕迹。部分线路距离较长且高差明显，建议提前评估体力、天气和缆车安排，行走时靠内侧通行，不在狭窄崖边长时间停留。",
	"shandong-mount-tai":                   "泰山的登山路线串联岱庙传统、山门、石刻、溪谷、古树和峰顶景观，历史上形成的祭祀文化让自然山岳具有鲜明的人文层次。中天门、十八盘、南天门和玉皇顶等节点可以帮助理解山势和登山节奏，沿途碑刻也记录了不同历史时期的游山传统。建议根据体力和时间选择徒步、接驳或索道组合，夜登、雨雪和高温时更要重视安全，保护石刻与山地环境。",
	"shandong-qufu-confucius-temple":       "曲阜三孔由孔庙、孔府和孔林组成，分别对应祭祀、家族居住与墓葬园林，完整展现了儒家文化长期制度化和地方化的过程。孔庙的中轴院落、碑刻和古树适合观察礼制建筑，孔府能够补充对家族管理和日常生活的认识，孔林则体现古代墓园与自然环境的结合。建议按三处遗产的功能差异安排时间，参观碑刻和古建筑时遵守保护规定，不触摸文物或擅自进入封闭区域。",
	"henan-longmen-grottoes":               "龙门石窟沿伊河两岸分布，造像、浮雕、题记和洞窟建筑记录了北魏、隋唐等时期石刻艺术的演进。奉先寺大佛、宾阳洞、莲花洞等区域在尺度、造像风格和保存状态上各有特点，适合边走边理解河谷地形与石窟选址的关系。光线、季节和人流会影响观赏体验，建议预留完整步行时间，使用讲解或数字资料辅助理解细节，并遵守禁止触摸、攀爬和使用闪光灯等保护要求。",
	"henan-shaolin-temple":                 "少林寺位于嵩山山麓，寺院、塔林、碑刻、山林和武术文化共同构成景区的历史层次。寺内建筑适合观察禅宗寺院的院落秩序，塔林展示不同年代的墓塔形制，嵩山步道则让宗教空间与自然山岳联系起来。武术表演和相关展陈能够补充对少林文化传播的认识，但不应把景区简化为表演场所。游览时注意寺院礼仪、山路安全和开放时段，合理安排寺院与山林线路。",
	"hubei-yellow-crane-tower":             "黄鹤楼依蛇山而建，楼阁、长江、武汉三镇城市景观和历代诗文共同形成其文化意义。登楼可以从高处观察长江走向、桥梁、城市建筑和山地关系，楼内展陈则帮助理解历代重修、文学书写与城市记忆。建议在楼体内部留意木构、彩绘和层层登临的空间感，再到周边园林和观景区域换角度观看。高峰期人流集中，拍摄和上下楼时注意通行秩序。",
	"hubei-wudang-mountains":               "武当山以道教宫观、古建筑群、峰林、森林和山地道路构成连续的文化景观，太和宫、紫霄宫、南岩宫和金顶等区域具有不同的建筑和地形特征。山路行进时可以观察宫观如何依山就势、借峰设景，也能理解宗教活动、建筑材料和自然环境之间的联系。景区范围较大，建议先确定核心线路，再根据体力增加步行段；山地天气变化快，注意防滑、保暖和交通接驳安排。",
	"hunan-zhangjiajie":                    "张家界国家森林公园以石英砂岩峰柱、峡谷、溪流和森林组成独特的立体山水，袁家界、金鞭溪、天子山等区域从不同高度展示峰林地貌。雾气、云层和光线会改变峰柱的远近关系，步行线路则能让游客观察岩壁纹理、植被和水体细节。景区面积较大，建议提前规划环保车、索道和步道衔接，避开拥挤时段；山路湿滑时不要靠近危险边缘，也不要在非开放区域攀爬。",
	"hunan-fenghuang-ancient-town":         "凤凰古城沿沱江展开，吊脚楼、古桥、城墙、码头和两岸街巷共同形成富有层次的水岸历史景观。白天适合观察木构建筑、巷道尺度和地方生活，傍晚或夜间则可以从江面和桥上看建筑灯光与山城轮廓。除了主街，支巷、河岸和周边村落也能帮助理解湘西传统聚落。游览时注意临水安全、商业区域的消费提示和居民生活边界，尽量放慢脚步而不是只追逐夜景。",
	"hunan-yueyang-tower":                  "岳阳楼位于洞庭湖畔，楼阁、湖面、城墙和文学传统共同构成景点的文化景观。登楼可以观察洞庭湖的开阔水面和岳阳城市边缘，楼内则可结合历代重修、题刻和诗文理解建筑为何长期成为江湖意象。建议把岳阳楼与洞庭湖岸线、古城街区安排为连续游览，预留天气变化带来的视野差异。临水和楼梯区域注意防滑，阅读碑刻和展陈时保持适当停留距离。",
	"guangdong-danxia-mountain":            "丹霞山以红色砂砾岩形成的峰林、赤壁、峡谷、洞穴和河流构成典型丹霞地貌，阳元石、长老峰、翔龙湖等区域体现不同的岩体形态和观景方式。沿步道行走可以观察岩层、风化纹理、植被与水系如何共同塑造山体，而高处观景台则适合理解整体地貌格局。建议根据天气选择登山和水上线路，穿防滑鞋并携带饮水；不要触摸、攀爬或刻画岩体，遵守自然保护区管理要求。",
	"guangdong-kaiping-diaolou":            "开平碉楼与村落把侨乡历史、乡村聚落、田园环境和中西合璧建筑结合在一起，碉楼既承担居住功能，也反映了华侨资金、建筑技术和地方防卫需求的交汇。自力村、马降龙等村落适合观察碉楼与水塘、村巷、农田之间的关系，建筑立面则可看到不同文化符号的并置。建议用村落步行代替只看单座建筑，尊重居民生活和私人空间，注意乡村道路交通与夏季户外防晒。",
	"guangxi-li-river":                     "漓江以喀斯特峰丛、江面倒影、竹林、田园和村落构成桂林山水的经典景观。水面视角适合连续观察峰体轮廓和河岸层次，岸上步道或村落则能补充对农业、民居和地方生活的认识。天气和水位会改变倒影、雾气与远山清晰度，建议在行程中留出弹性，不把体验只限定为拍摄同一张构图。乘船、漂流或沿岸活动要服从安全调度，保护河岸植被并减少塑料垃圾。",
	"guangxi-longji-rice-terraces":         "龙脊梯田沿山势层层展开，水田、山林、村寨和步道共同构成适应坡地环境的农业景观。灌水期可以看到镜面般的田块，生长季呈现连续绿色，收获前后则会出现更丰富的金色层次；不同观景台适合观察梯田整体曲线和村寨细节。游览时应把梯田当作仍在使用的生产空间，沿指定道路行走，不踩踏田埂、不随意进入农田，并根据山地坡度、天气和住宿位置合理安排步行线路。",
	"hainan-wuzhizhou-island":              "蜈支洲岛以清澈海湾、礁石、热带植被和岛屿步道组成滨海自然景观，海水颜色会随天气、潮汐和光线发生变化。沿岸行走可以观察海岸地貌和热带植物，水上活动则让人从海面角度理解岛屿轮廓与湾区环境。建议提前确认船班、天气和项目开放情况，注意防晒、补水和个人物品防水；珊瑚、贝壳和海岸生物属于脆弱生态，不捕捉、不带走、不踩踏。",
	"hainan-yalong-bay":                    "亚龙湾以弧形海湾、沙滩、浅海、热带林带和滨海度假空间形成连续的热带海岸景观。不同时间段的海水色泽、沙滩光影和远山轮廓差异明显，清晨适合安静散步，傍晚适合观察海岸线与天空变化。游览可以把海滩、沿岸绿地和附近热带生态空间结合，不必只停留在单一拍摄点。下水和水上活动要遵守安全区域与天气提示，保护沙滩清洁并避免破坏海岸植被。",
	"chongqing-wulong-karst":               "武隆喀斯特以天生三桥、天坑、地缝、溶洞和峡谷构成具有强烈空间变化的地质景观。天生三桥适合观察巨大天然桥体和峡谷尺度，龙水峡地缝则呈现地下水侵蚀形成的狭长空间，沿线瀑布、岩壁和植被会不断改变视线。景区步道和高差较多，建议按接驳车、步道和电梯等环节规划体力，雨天注意落石和湿滑，听从现场管理，不靠近封闭或警示区域。",
	"chongqing-hongya-cave":                "洪崖洞依山就势布置，吊脚楼式建筑、层叠街区、嘉陵江岸线和重庆城市高差共同形成独特的立体城市景观。白天可以观察山城建筑如何利用坡地和高层步行系统，夜间则能看到灯光、桥梁、江面和城市天际线叠加出的视觉效果。建议从不同高程的入口和江岸寻找视角，注意节假日人流密集、楼层路线复杂和临江区域安全。这里同时是商业与公共通行空间，应保持通道畅通。",
	"sichuan-jiuzhaigou":                   "九寨沟由彩色湖泊、钙华滩、瀑布、雪山、森林和高原河谷组成，水体颜色与透明度会随深度、矿物、天气和光线变化。树正、诺日朗、五花海、长海等区域展现不同的湖泊和瀑布形态，环保车与木栈道让游客可以在较大范围内连续观察生态景观。景区生态脆弱，必须沿指定路线行走，不投喂、不触碰水体和植被；高原天气变化快，提前准备保暖、防晒和适应高海拔的节奏。",
	"sichuan-mount-emei":                   "峨眉山把佛教寺院、古道、森林、溪流、云海和高山生态结合在一条垂直山地景观中，从山麓到高处可以感受植被和气候随海拔变化。报国寺、万年寺、清音阁和金顶等区域分别体现寺院文化、溪谷景观和高山视野，适合按线路分段体验。山地距离和高差较大，建议合理利用接驳交通并留出休息时间；尊重寺院礼仪和野生动物，不在步道外穿行，也要关注雾、雨和低温天气。",
	"sichuan-leshan-buddha":                "乐山大佛依凌云山崖壁开凿，头部、肩部、手部和足部的尺度可以从江面、山路和对岸不同角度理解。大佛所在位置面对岷江、青衣江和大渡河汇流区域，石刻工程与水运、山地和地方宗教文化之间存在密切联系。游览可以结合山上近观、栈道视角和江面远观，比较不同距离对尺度感的影响。临江和临崖路线注意安全，遵守文物保护规定，不触摸或攀爬石刻。",
	"guizhou-huangguoshu-waterfall":        "黄果树瀑布所在区域由瀑布、河谷、溶洞、天生桥和湿润的喀斯特植被共同构成，水量、季节和天气会显著影响瀑布的声势与水雾。水帘洞、犀牛潭和周边步道可以从不同高度和距离观察水流，既能感受大景，也能看到岩壁、苔藓和溪流细节。雨季地面湿滑，建议穿防滑鞋并保护相机和手机；按照指定方向游览，不跨越护栏，不在水流危险区域停留。",
	"guizhou-xijiang-miao-village":         "西江千户苗寨依山谷展开，吊脚楼、风雨桥、梯田、河流和山地道路共同构成苗族聚落景观。白天适合从村巷和观景台观察民居层叠、木构细节与农业环境，夜间则可以看到山坡灯火和谷地轮廓。银饰、歌舞、饮食和节庆等文化内容能够补充对村寨生活的认识，但应以尊重当地居民和真实生活为前提。山地步道坡度较大，夜间拍摄注意脚下、交通和临水安全。",
	"yunnan-stone-forest":                  "石林由石灰岩峰、柱、芽和洞穴组成，岩溶长期侵蚀形成的尖锐轮廓让步道在石峰之间不断转折。大小石林适合观察不同尺度的岩体组合，彝族撒尼文化、传说和地方生活则为自然景观增加了人文解释。游览时可以从近距离看岩石纹理，也可以登高理解石林在地貌中的整体分布。园区道路多为石材铺装，注意防滑和防晒，不攀爬岩体、不刻画石面，并遵守景区讲解和保护规则。",
	"yunnan-lijiang-old-town":              "丽江古城以水系、石板街、木构民居、纳西族文化和高原山城环境形成独特的生活型历史城区。沿河道和支巷行走，可以观察水车、水渠、院落、店铺和民居之间的关系，四方街及周边公共空间则体现古城的商业与社交功能。建议避开只走主街的路线，关注清晨生活、夜间灯火和建筑细节；高原日照强、昼夜温差较大，保持适度步行节奏，尊重居民生活和古城建筑。",
	"yunnan-dali-ancient-town":             "大理古城位于苍山与洱海之间，城墙、城门、街巷、白族建筑、地方市集和远山湖景共同构成高原历史城区。古城内部适合慢行观察照壁、门楼、院落和街道尺度，离开主街后可以看到更接近日常生活的社区环境。苍山、洱海和周边村落为古城提供了完整的山水背景，适合组合成多日线路。高原紫外线较强，注意防晒和补水，进入居民区域时保持安静并尊重地方习俗。",
	"tibet-potala-palace":                  "布达拉宫依山而建，白宫、红宫、宫墙、台阶和高原城市背景共同形成极具辨识度的建筑景观。建筑群内部包含殿堂、佛塔、壁画、经书和宗教器物，外部视角则能观察宫殿体量与拉萨地形之间的关系。参观通常需要预约并遵循限流和安检安排，进入宗教空间时应遵守着装、摄影和行走要求。拉萨海拔较高，建议预留适应时间，减少剧烈运动，按预约时间有序参观。",
	"tibet-namtso":                         "纳木错是高原湖泊、雪山、草原和开阔天空共同构成的自然景观，湖面色彩和远山轮廓会随光线、云层与风力快速变化。湖岸视野开阔，适合观察湖山比例、岸线形态和高原植被，也能感受高海拔环境下景观的辽阔与寂静。前往高原湖区需要充分考虑海拔、交通距离和天气，不要靠近危险岸段或在未开放区域露营；注意保暖、防晒、补水和垃圾带离，尊重当地生态与文化环境。",
	"shaanxi-terracotta-army":              "秦始皇帝陵博物院以兵马俑坑、铜车马和陵园考古成果为核心，陶俑的阵列、姿态、发式、铠甲和面部细节展示了秦代军队与手工业组织的复杂程度。一号坑适合感受军阵规模，二号坑和三号坑能够补充对不同编制和考古过程的认识，展馆文物则帮助理解秦代交通、礼制和技术。室内光线和保护条件严格，参观时不触摸、不使用闪光灯，按照展厅路线和人流节奏观察细节。",
	"shaanxi-mount-hua":                    "华山以花岗岩峰体、陡峭山脊、崖壁、峡谷和栈道构成高辨识度的山岳景观，东峰、南峰、西峰、北峰和中峰从不同方向展现山势。长空栈道、苍龙岭等路段具有较强挑战性，实际开放和安全要求会随天气变化。登山前应评估体力、恐高情况和路线时间，准备防滑鞋、饮水和保暖用品，严格使用护栏和安全设施。山上观景不要追求危险角度，保持与崖边及其他游客的安全距离。",
	"shaanxi-xian-city-wall":               "西安城墙保留了较完整的明代城防体系，城门、箭楼、角楼、女墙、护城河和城内街区共同构成古城空间。站在城墙上可以同时观察城门结构、城市道路、钟鼓楼方向和现代建筑的层叠关系，骑行则能更直观地感受城墙周长和不同门区的差异。建议选择天气舒适的时段完整走一段或骑行一圈，注意城墙路面、上下坡和车辆规则，不触摸或攀爬保护设施。",
	"gansu-mogao-caves":                    "莫高窟保存了跨越多个历史时期的洞窟、壁画、彩塑、题记和宗教艺术，是理解敦煌与丝绸之路文化交流的重要场所。洞窟内部的供养人、经变画、飞天、建筑图像和生活场景提供了丰富的历史信息，数字展示则帮助游客在保护文物的前提下建立整体认识。参观需要预约并遵守洞窟开放、人数和摄影规定，不能触摸壁画和彩塑；敦煌气候干燥，行程中注意补水、防晒和保护相机设备。",
	"gansu-zhangye-danxia":                 "张掖七彩丹霞由不同颜色和硬度的岩层经过长期沉积、抬升和侵蚀形成，丘陵、沟壑、峰墙和色带在观景台之间呈现连续变化。不同时间的太阳高度会改变红、黄、橙和灰色地层的对比，适合在多个观景点比较近景纹理和远景整体构图。景区地表十分脆弱，必须乘坐规定交通并沿栈道行走，不踩踏岩层、不带走石块。高温、风沙和日照较强时应准备饮水、防晒和护眼用品。",
	"qinghai-qinghai-lake":                 "青海湖由辽阔湖面、环湖草原、湿地、雪山和牧业景观共同构成高原湖泊生态空间，湖水颜色会随着天气和光线在蓝、青和灰之间变化。环湖行驶时可以观察湖岸、草场、鸟类栖息环境和远山之间的尺度关系，不同季节的花草、候鸟和风力会带来不同体验。高原旅行需要安排适应时间，注意防晒、保暖、补水和交通安全；不要靠近鸟类繁殖地或驶入封闭草场，带走垃圾并尊重当地牧民生活。",
	"qinghai-chaka-salt-lake":              "茶卡盐湖以盐湖水面、盐晶、盐滩和高原天空形成镜面般的视觉效果，天气、风力和水位会直接影响倒影清晰度。盐湖小火车、栈道和盐雕等元素可以帮助游客从不同距离观察盐业地貌与旅游设施之间的关系。盐滩和浅水区看似平缓，实际可能松软或含有尖锐盐晶，应按照开放路线行走，避免踩踏保护区域。高原日照强、昼夜温差大，注意防晒、保暖和鞋袜清洁。",
	"ningxia-shapotou":                     "沙坡头位于黄河、沙漠、绿洲和山地交汇处，能够在较短距离内观察河流冲积、荒漠地貌和人工治沙形成的复合景观。沿黄河岸线可以感受水面与沙丘的对比，登上沙丘则能看到绿洲、铁路、河谷和远山的整体关系。滑沙、缆车等活动具有较强体验性，但应根据天气和个人身体状况选择。沙漠风沙和日照较强，注意护眼、防晒、补水，不破坏固沙植被和防护设施。",
	"ningxia-western-xia-tombs":            "西夏王陵分布在贺兰山麓的荒漠平原上，夯土陵台、陵区道路、山体背景和出土文物共同呈现西夏王朝的历史记忆。陵墓建筑在开阔地貌中具有强烈的尺度感，适合观察夯土结构、遗址布局与山地荒漠环境的关系。参观时可以结合博物馆和遗址区理解西夏文字、宗教、贸易与多民族交流，不攀爬遗址、不触碰夯土。夏季炎热、遮阴较少，应提前准备饮水、防晒和适合户外行走的鞋物。",
	"xinjiang-kanas":                       "喀纳斯以高山湖泊、针叶林、河谷、草原和图瓦村落共同构成新疆北部的山地景观，不同季节的森林色彩、湖面云影和雪线变化十分明显。湖区观景台适合从高处理解湖泊与山谷的关系，村落和河谷步道则能补充对地方生活、木构建筑和生态环境的认识。景区范围较大，建议合理安排区间车和步行线路；山区天气变化快，注意保暖、防晒、补水和防滑，不在野生动物或封闭区域附近停留。",
	"xinjiang-tianchi":                     "天山天池是火山活动和冰川地貌共同影响下形成的高山湖泊，湖面、雪峰、针叶林和山地草甸构成具有垂直层次的景观。晴天可以远眺博格达峰，云雾天气则更适合观察湖岸森林和近距离山体纹理。游览可以安排湖岸步行、观景台和周边山地线路，但高海拔和天气变化会影响体力与视野。注意保暖、防晒和防滑，遵守景区交通、步道和生态保护规定，不随意采摘植物或投喂动物。",
	"xinjiang-kashgar-old-city":            "喀什古城以传统街巷、土木民居、院落、市集、清真寺和手工艺生活构成丝绸之路节点城市的历史肌理。沿街行走可以观察门窗、色彩、屋顶、巷道和商业空间如何适应干燥气候与社区生活，老城更新后的公共空间也展示传统风貌与现代使用之间的关系。建议早晚分时段游览，尊重礼拜场所、居民住宅和当地拍摄习惯；市场购物注意价格和交通，夏季注意防晒、补水与步行休息。",
}

func richDetail(item attraction) string {
	note := detailedNotes[item.Slug]
	if note == "" {
		note = "可以围绕景点的核心建筑、自然环境、地方文化和整体空间关系展开游览，建议在主要观景点之外安排步行和停留时间，观察细节而不是只进行短暂停留。"
	}
	categoryAdvice := map[string]string{
		"自然景观": "自然景观受天气、季节和开放条件影响较大，出发前应确认路线和安全提示，行走时保护植被与地貌，带走个人垃圾。",
		"世界遗产": "作为重要文化或自然遗产，参观时应遵守预约、限流、摄影和文物保护规定，把保护遗产本身作为游览的一部分。",
		"历史建筑": "历史建筑和遗址对环境、触碰和人流较敏感，建议按照开放路线参观，注意建筑细节，不攀爬、不刻画、不进入封闭区域。",
		"城市景观": "城市景观适合结合步行、公共空间和周边街区体验，建议错开高峰时段，并注意道路、滨水区域和人流密集处的安全。",
		"园林":   "园林空间适合慢行和多角度观察，建议留意借景、框景、水系、植物与建筑之间的关系，不在狭窄路径长时间停留。",
		"宗教文化": "宗教场所需要保持安静并尊重当地仪式、着装和摄影要求，进入殿堂或礼拜空间前先确认相关规定。",
		"人文景观": "人文景观仍然与当地居民的日常生活相连，游览时应尊重社区秩序、地方习俗和私人空间。",
		"历史遗址": "遗址地表和残存建筑通常较为脆弱，应沿指定路线参观，不触碰、不攀爬，并结合展陈或讲解理解遗址背景。",
	}
	advice := categoryAdvice[item.Category]
	if advice == "" {
		advice = "建议根据天气、体力和开放安排合理规划路线，遵守现场管理规定并保护景点环境。"
	}
	return limitUTF8Runes(fmt.Sprintf("%s\n\n景点概览：%s位于%s%s，是%s类型的代表性目的地。%s\n\n主要看点：%s\n\n游览建议：建议将核心看点、周边步行路线和休息节点组合起来，预留观察建筑、地貌、植被、展陈或地方生活细节的时间。若行程包含登山、滨水、高原、沙漠或人流密集区域，应根据当天状况调整节奏，不为追求完整打卡而忽略安全。%s\n\n补充安排：出发前核对开放、预约、交通和天气；按路线安排返程，拍摄时尊重文物、宗教场所和居民，带走垃圾并优先使用公共交通。\n\n体验提示：建议先用地图确认景区入口、核心区域和返程方向，再按天气与体力调整顺序。上午通常适合参观建筑、遗址和展馆，光线稳定时也便于观察细节；下午可把时间留给步道、岸线或开阔观景处。长距离出行应提前确认交通衔接、饮水、厕所、寄存和休息点，老人儿童同行时减少连续赶路。拍摄以不影响他人为前提，遵守景区对无人机、闪光灯和商业摄影的规定。\n\n行前准备：请在出发当天再次核对官方公告，尤其留意预约时段、临时闭园、天气预警与接驳调整。准备合适的鞋服、雨具、充电设备和身份证件；山区、海边与高原地点还应为温差、强紫外线和通信不稳留出余量。遇到拥挤或天气变化时，宁可缩短路线，也不要脱离开放区域或影响同行者的安全。", item.Detail, item.Name, item.Province, item.City, item.Category, note, note, advice), 2048)
}

func limitUTF8Runes(value string, maxRunes int) string {
	if utf8.RuneCountInString(value) <= maxRunes {
		return value
	}
	return string([]rune(value)[:maxRunes])
}

func collect(outDir string) error {
	if len(attractions) == 0 {
		return errors.New("no attractions configured")
	}
	if err := os.MkdirAll(filepath.Join(outDir, "images"), 0755); err != nil {
		return fmt.Errorf("create image directory: %w", err)
	}

	client := &http.Client{Timeout: 90 * time.Second}
	hotPlaces := make([]hotPlace, len(attractions))
	placeCatalog := make([]placeCatalogItem, len(attractions))
	sources := make([]sourceRecord, len(attractions))
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 4)
	errorsCh := make(chan error, len(attractions))

	for index, item := range attractions {
		index, item := index, item
		wg.Add(1)
		go func() {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			hotIdentity := stableUUID("hot-place/" + item.Slug)
			imageIdentity := stableUUID("place-image/" + item.Slug)
			placeIdentity := stableUUID("place/" + item.Slug)
			candidate, err := searchCommons(client, item.SearchQuery)
			if err != nil {
				imageName := item.Slug + ".jpg"
				imagePath := filepath.Join(outDir, "images", imageName)
				if _, statErr := os.Stat(imagePath); statErr == nil {
					writeCollectedWithoutSource(index, item, outDir, hotPlaces, placeCatalog, sources)
					fmt.Printf("collected %02d/%02d %s (source metadata deferred)\n", index+1, len(attractions), item.Name)
					return
				}
				errorsCh <- fmt.Errorf("%s: search image: %w", item.Slug, err)
				return
			}
			imageName := item.Slug + ".jpg"
			imagePath := filepath.Join(outDir, "images", imageName)
			encoded, readErr := os.ReadFile(imagePath)
			selectedCandidate := candidate
			if readErr != nil {
				var downloadErr error
				foundImage := false
				candidates := append([]commonsCandidate{candidate}, candidate.Alternatives...)
				for _, option := range candidates {
					imageBytes, candidateErr := download(client, option.ProxyURL, option.ThumbURL, imageProxyURL(option.RedirectURL), option.RedirectURL, option.RedirectAltURL, option.ImageURL)
					if candidateErr != nil {
						downloadErr = candidateErr
						continue
					}
					encoded, err = normalizeImage(imageBytes)
					if err != nil {
						downloadErr = err
						continue
					}
					selectedCandidate = option
					foundImage = true
					break
				}
				if !foundImage {
					errorsCh <- fmt.Errorf("%s: download image %s: %w", item.Slug, candidate.Title, downloadErr)
					return
				}
				if err := os.WriteFile(imagePath, encoded, 0644); err != nil {
					errorsCh <- fmt.Errorf("%s: save image: %w", item.Slug, err)
					return
				}
			}

			hotPlaces[index] = hotPlace{
				HotPlaceUniqueID: uint32(index + 1),
				HotPlaceIdentity: hotIdentity,
				RecommendTitle:   item.RecommendTitle,
				RecommandDetail:  richDetail(item),
				PlaceImageItemID: imageIdentity,
				PlaceIdentity:    placeIdentity,
			}
			placeCatalog[index] = placeCatalogItem{PlaceIdentity: placeIdentity}
			sources[index] = sourceRecord{
				Slug:               item.Slug,
				ImageFile:          filepath.ToSlash(filepath.Join("images", imageName)),
				CommonsPage:        selectedCandidate.PageURL,
				OriginalImageURL:   selectedCandidate.ImageURL,
				DownloadedImageURL: selectedCandidate.ThumbURL,
				License:            selectedCandidate.License,
				Artist:             selectedCandidate.Artist,
				Attribution:        selectedCandidate.Credit,
				SourceWidth:        selectedCandidate.Width,
				SourceHeight:       selectedCandidate.Height,
				OutputWidth:        imageWidth,
				OutputHeight:       imageHeight,
			}
			fmt.Printf("collected %02d/%02d %s\n", index+1, len(attractions), item.Name)
		}()
	}
	wg.Wait()
	close(errorsCh)
	var collectedErrors []error
	for err := range errorsCh {
		collectedErrors = append(collectedErrors, err)
	}
	if len(collectedErrors) > 0 {
		sort.Slice(collectedErrors, func(i, j int) bool { return collectedErrors[i].Error() < collectedErrors[j].Error() })
		return fmt.Errorf("collection failed for %d attractions:\n- %s", len(collectedErrors), joinErrors(collectedErrors))
	}

	if err := writeJSON(filepath.Join(outDir, "hot_places.json"), hotPlaces); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(outDir, "place_catalog.json"), placeCatalog); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(outDir, "image_sources.json"), sources); err != nil {
		return err
	}
	return syncAmapPlaces(outDir)
}

func writeCollectedWithoutSource(index int, item attraction, outDir string, hotPlaces []hotPlace, placeCatalog []placeCatalogItem, sources []sourceRecord) {
	hotIdentity := stableUUID("hot-place/" + item.Slug)
	imageIdentity := stableUUID("place-image/" + item.Slug)
	placeIdentity := stableUUID("place/" + item.Slug)
	imageName := item.Slug + ".jpg"
	hotPlaces[index] = hotPlace{
		HotPlaceUniqueID: uint32(index + 1),
		HotPlaceIdentity: hotIdentity,
		RecommendTitle:   item.RecommendTitle,
		RecommandDetail:  richDetail(item),
		PlaceImageItemID: imageIdentity,
		PlaceIdentity:    placeIdentity,
	}
	placeCatalog[index] = placeCatalogItem{PlaceIdentity: placeIdentity}
	sources[index] = sourceRecord{
		Slug:         item.Slug,
		ImageFile:    filepath.ToSlash(filepath.Join("images", imageName)),
		CommonsPage:  commonsSearchURL(item.SearchQuery),
		License:      "Wikimedia Commons; refresh source metadata from the search page before redistribution",
		OutputWidth:  imageWidth,
		OutputHeight: imageHeight,
	}
}

func stableUUID(value string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("https://evercare.example/hotplaces/"+value)).String()
}

func enrichExisting(outDir string) error {
	var hotPlaces []hotPlace
	if err := readJSON(filepath.Join(outDir, "hot_places.json"), &hotPlaces); err != nil {
		return err
	}
	if len(hotPlaces) != len(attractions) {
		return fmt.Errorf("expected %d HotPlace records, got %d", len(attractions), len(hotPlaces))
	}
	for index := range hotPlaces {
		hotPlaces[index].RecommandDetail = richDetail(attractions[index])
	}
	if err := writeJSON(filepath.Join(outDir, "hot_places.json"), hotPlaces); err != nil {
		return err
	}
	return nil
}

var curatedCategoryNames = map[string]string{
	"beijing-forbidden-city":               "历史文化;皇家建筑;博物馆",
	"beijing-great-wall":                   "历史文化;长城古迹;山岳景观",
	"beijing-temple-of-heaven":             "历史文化;礼制建筑;古典园林",
	"tianjin-ancient-culture-street":       "历史文化;历史街区;民俗商业",
	"hebei-chengde-mountain-resort":        "历史文化;皇家园林;宫殿建筑",
	"hebei-shanhaiguan":                    "历史文化;长城古迹;关隘建筑",
	"shanxi-yungang-grottoes":              "历史文化;石窟艺术;佛教文化",
	"shanxi-pingyao-ancient-city":          "历史文化;古城古镇;晋商文化",
	"inner-mongolia-xiangshawan":           "自然风光;沙漠景观;户外体验",
	"inner-mongolia-hulunbuir":             "自然风光;草原景观;牧业文化",
	"liaoning-shenyang-imperial-palace":    "历史文化;皇家建筑;博物馆",
	"liaoning-dalian-xinghai-square":       "城市景观;城市广场;滨海风光",
	"jilin-changbai-mountain":              "自然风光;山岳景观;火山地貌",
	"heilongjiang-saint-sophia":            "历史文化;宗教建筑;城市地标",
	"shanghai-bund":                        "城市景观;历史建筑;滨江风光",
	"shanghai-yu-garden":                   "历史文化;古典园林;江南建筑",
	"jiangsu-humble-administrators-garden": "历史文化;古典园林;江南建筑",
	"jiangsu-sun-yat-sen-mausoleum":        "历史文化;纪念建筑;山林景观",
	"zhejiang-west-lake":                   "自然风光;湖泊景观;人文胜迹",
	"zhejiang-mount-putuo":                 "宗教文化;佛教名山;海岛风光",
	"anhui-huangshan":                      "自然风光;山岳景观;地质奇观",
	"anhui-hongcun":                        "历史文化;古村落;徽派建筑",
	"fujian-gulangyu":                      "海岛风光;历史街区;建筑人文",
	"fujian-wuyi-mountains":                "自然风光;丹霞地貌;茶文化",
	"jiangxi-lushan":                       "自然风光;山岳景观;避暑胜地",
	"jiangxi-sanqing-mountain":             "自然风光;山岳景观;道教文化",
	"shandong-mount-tai":                   "山岳景观;历史文化;五岳名山",
	"shandong-qufu-confucius-temple":       "历史文化;儒家文化;古建筑群",
	"henan-longmen-grottoes":               "历史文化;石窟艺术;佛教文化",
	"henan-shaolin-temple":                 "宗教文化;佛教寺院;武术文化",
	"hubei-yellow-crane-tower":             "历史文化;历史名楼;城市地标",
	"hubei-wudang-mountains":               "宗教文化;道教名山;古建筑群",
	"hunan-zhangjiajie":                    "自然风光;峰林峡谷;森林公园",
	"hunan-fenghuang-ancient-town":         "历史文化;古城古镇;民族风情",
	"hunan-yueyang-tower":                  "历史文化;历史名楼;湖泊景观",
	"guangdong-danxia-mountain":            "自然风光;丹霞地貌;地质奇观",
	"guangdong-kaiping-diaolou":            "历史文化;侨乡建筑;古村落",
	"guangxi-li-river":                     "自然风光;河流山水;喀斯特地貌",
	"guangxi-longji-rice-terraces":         "自然风光;梯田景观;民族风情",
	"hainan-wuzhizhou-island":              "海岛风光;海滨度假;水上活动",
	"hainan-yalong-bay":                    "海滨风光;沙滩海湾;休闲度假",
	"chongqing-wulong-karst":               "自然风光;喀斯特地貌;峡谷天坑",
	"chongqing-hongya-cave":                "城市景观;民俗街区;山城夜景",
	"sichuan-jiuzhaigou":                   "自然风光;湖泊瀑布;高原生态",
	"sichuan-mount-emei":                   "宗教文化;佛教名山;山岳景观",
	"sichuan-leshan-buddha":                "历史文化;石刻艺术;佛教文化",
	"guizhou-huangguoshu-waterfall":        "自然风光;瀑布景观;喀斯特地貌",
	"guizhou-xijiang-miao-village":         "民族文化;民族村寨;传统建筑",
	"yunnan-stone-forest":                  "自然风光;喀斯特地貌;民族文化",
	"yunnan-lijiang-old-town":              "历史文化;古城古镇;民族文化",
	"yunnan-dali-ancient-town":             "历史文化;古城古镇;民族文化",
	"tibet-potala-palace":                  "宗教文化;宫殿建筑;历史遗产",
	"tibet-namtso":                         "自然风光;高原湖泊;雪山草原",
	"shaanxi-terracotta-army":              "历史文化;考古遗址;博物馆",
	"shaanxi-mount-hua":                    "自然风光;山岳景观;五岳名山",
	"shaanxi-xian-city-wall":               "历史文化;城防古迹;古城建筑",
	"gansu-mogao-caves":                    "历史文化;石窟艺术;丝路文化",
	"gansu-zhangye-danxia":                 "自然风光;丹霞地貌;地质奇观",
	"qinghai-qinghai-lake":                 "自然风光;高原湖泊;草原生态",
	"qinghai-chaka-salt-lake":              "自然风光;盐湖景观;高原风光",
	"ningxia-shapotou":                     "自然风光;沙漠景观;黄河风光",
	"ningxia-western-xia-tombs":            "历史文化;陵墓遗址;西夏文化",
	"xinjiang-kanas":                       "自然风光;高山湖泊;森林草原",
	"xinjiang-tianchi":                     "自然风光;高山湖泊;雪山森林",
	"xinjiang-kashgar-old-city":            "历史文化;历史街区;民族风情",
}

func refreshCategories(outDir string) error {
	var places []placeCatalogItem
	if err := readJSON(filepath.Join(outDir, "place_catalog.json"), &places); err != nil {
		return err
	}
	if len(places) != len(attractions) {
		return fmt.Errorf("expected %d place_catalog records, got %d", len(attractions), len(places))
	}
	for index, attraction := range attractions {
		if places[index].PlaceIdentity != stableUUID("place/"+attraction.Slug) {
			return fmt.Errorf("place_catalog[%d] does not match attraction %q", index, attraction.Slug)
		}
		categoryName, ok := curatedCategoryNames[attraction.Slug]
		if !ok || strings.TrimSpace(categoryName) == "" {
			return fmt.Errorf("missing curated category for attraction %q", attraction.Slug)
		}
		if err := validateCuratedCategoryName(categoryName); err != nil {
			return fmt.Errorf("invalid curated category for attraction %q: %w", attraction.Slug, err)
		}
		places[index].CategoryName = categoryName
	}
	return writeJSON(filepath.Join(outDir, "place_catalog.json"), places)
}

func syncAmapPlaces(outDir string) error {
	var places []placeCatalogItem
	if err := readJSON(filepath.Join(outDir, "place_catalog.json"), &places); err != nil {
		return err
	}
	if len(places) != len(attractions) {
		return fmt.Errorf("expected %d place_catalog records, got %d", len(attractions), len(places))
	}

	ctx := context.Background()
	providerIDs := make(map[string]string, len(places))
	for index, attraction := range attractions {
		placeIdentity := strings.TrimSpace(places[index].PlaceIdentity)
		if _, err := uuid.Parse(placeIdentity); err != nil {
			return fmt.Errorf("place_catalog[%d] has invalid PlaceIdentity %q", index, placeIdentity)
		}

		selected, _, ok, err := findAmapPlace(ctx, attraction)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("no confident Amap POI match for %s in %s", attraction.Name, attraction.City)
		}
		if priorName, exists := providerIDs[selected.ProviderPlaceID]; exists {
			return fmt.Errorf("Amap POI %s was selected for both %s and %s", selected.ProviderPlaceID, priorName, attraction.Name)
		}
		providerIDs[selected.ProviderPlaceID] = attraction.Name
		categoryName, exists := curatedCategoryNames[attraction.Slug]
		if !exists {
			return fmt.Errorf("missing curated category for attraction %q", attraction.Slug)
		}
		if err := validateCuratedCategoryName(categoryName); err != nil {
			return fmt.Errorf("invalid curated category for attraction %q: %w", attraction.Slug, err)
		}
		places[index] = placeCatalogItem{
			PlaceIdentity:   placeIdentity,
			ProviderName:    define.PlaceProviderNameDefault,
			ProviderPlaceID: selected.ProviderPlaceID,
			Name:            selected.Name,
			CategoryCode:    selected.CategoryCode,
			CategoryName:    categoryName,
			FullAddress:     selected.FullAddress,
			Province:        selected.ProvinceName,
			City:            selected.CityName,
			District:        selected.DistrictName,
			AdCode:          selected.AdCode,
			Longitude:       selected.Longitude,
			Latitude:        selected.Latitude,
		}
	}

	if err := writeJSON(filepath.Join(outDir, "place_catalog.json"), places); err != nil {
		return err
	}
	return nil
}

func findAmapPlace(ctx context.Context, attraction attraction) (utils.AmapPlace, int, bool, error) {
	candidateSummaries := make([]string, 0, 25)
	for _, keyword := range amapSearchKeywords(attraction.Name) {
		result, err := utils.SearchAmapPlaces(ctx, utils.AmapPlaceSearchOptions{
			Keywords:  keyword,
			City:      attraction.City,
			CityLimit: true,
			Page:      1,
			PageSize:  3,
		})
		if err != nil {
			return utils.AmapPlace{}, 0, false, fmt.Errorf("search Amap POI for %s (%s): %w", attraction.Name, attraction.City, err)
		}
		if selected, score, ok := selectAmapPlace(attraction, result.Places); ok {
			return selected, score, true, nil
		}
		for _, candidate := range result.Places {
			candidateSummaries = append(candidateSummaries, fmt.Sprintf("%s (%s)", candidate.Name, candidate.ProviderPlaceID))
		}
	}
	return utils.AmapPlace{}, 0, false, fmt.Errorf(
		"no confident Amap POI match for %s in %s; candidates: %s",
		attraction.Name,
		attraction.City,
		strings.Join(candidateSummaries, "; "),
	)
}

func amapSearchKeywords(name string) []string {
	keywords := []string{name}
	if alias, found := amapPlaceAliases[name]; found {
		keywords = append(keywords, alias...)
	}
	return keywords
}

var amapPlaceAliases = map[string][]string{
	"呼伦贝尔草原":  {"呼伦贝尔大草原"},
	"沈阳故宫":    {"沈阳故宫博物院"},
	"大连星海广场":  {"星海广场"},
	"豫园":      {"上海豫园"},
	"曲阜三孔":    {"曲阜明故城(三孔)旅游区"},
	"开平碉楼与村落": {"开平碉楼文化旅游区"},
	"武隆喀斯特":   {"武隆喀斯特旅游区"},
	"洪崖洞":     {"洪崖洞民俗风貌区"},
	"黄果树瀑布":   {"黄果树景区-黄果树大瀑布"},
	"石林":      {"石林风景区"},
	"纳木错":     {"纳木措国家风景区"},
	"沙坡头":     {"沙坡头旅游景区"},
}

func selectAmapPlace(attraction attraction, candidates []utils.AmapPlace) (utils.AmapPlace, int, bool) {
	bestScore := 0
	bestDistance := math.Inf(1)
	var best utils.AmapPlace
	for _, candidate := range candidates {
		score := 0
		for _, expectedName := range amapSearchKeywords(attraction.Name) {
			score = maxInt(score, amapNameScore(expectedName, candidate.Name))
		}
		if score == 0 {
			continue
		}
		distance := haversineKilometres(attraction.Longitude, attraction.Latitude, candidate.Longitude, candidate.Latitude)
		if score > bestScore || (score == bestScore && distance < bestDistance) {
			best, bestScore, bestDistance = candidate, score, distance
		}
	}
	// Only accept exact equality after removing generic scenic-area suffixes.
	// A containment-only match can select a nearby hotel, station, or visitor
	// centre instead of the attraction itself.
	return best, bestScore, bestScore == 100
}

func maxInt(first, second int) int {
	if first > second {
		return first
	}
	return second
}

func amapNameScore(expected string, actual string) int {
	expected = normalizeAmapPlaceName(expected)
	actual = normalizeAmapPlaceName(actual)
	if expected == "" || actual == "" {
		return 0
	}
	if expected == actual {
		return 100
	}
	if strings.Contains(actual, expected) || strings.Contains(expected, actual) {
		if utf8.RuneCountInString(expected) >= 4 && utf8.RuneCountInString(actual) >= 4 {
			return 80
		}
	}
	return 0
}

func normalizeAmapPlaceName(value string) string {
	replacer := strings.NewReplacer(
		" ", "", "　", "", "-", "", "·", "", "・", "", "（", "", "）", "", "(", "", ")", "",
	)
	value = replacer.Replace(strings.TrimSpace(value))
	for _, suffix := range []string{"风景名胜区", "国家森林公园", "国家地质公园", "旅游风景区", "文化旅游区", "生态旅游区", "旅游度假区", "风景区", "旅游区", "景区", "公园", "景点"} {
		value = strings.TrimSuffix(value, suffix)
	}
	return value
}

func haversineKilometres(longitudeA, latitudeA, longitudeB, latitudeB float64) float64 {
	const earthRadiusKM = 6371.0088
	toRadians := math.Pi / 180
	dLat := (latitudeB - latitudeA) * toRadians
	dLon := (longitudeB - longitudeA) * toRadians
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(latitudeA*toRadians)*math.Cos(latitudeB*toRadians)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return earthRadiusKM * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

var commonsRequestMu sync.Mutex

func searchCommons(client *http.Client, query string) (commonsCandidate, error) {
	// Commons applies a fairly small anonymous-client rate limit. Keep the
	// metadata requests serialized even while image downloads run in parallel.
	commonsRequestMu.Lock()
	defer commonsRequestMu.Unlock()

	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*attempt) * time.Second)
		}
		candidate, err := searchCommonsOnce(client, query)
		if err == nil {
			return candidate, nil
		}
		lastErr = err
		if !strings.Contains(err.Error(), "HTTP 429") {
			return commonsCandidate{}, err
		}
	}
	return commonsCandidate{}, lastErr
}

func searchCommonsOnce(client *http.Client, query string) (commonsCandidate, error) {
	time.Sleep(750 * time.Millisecond)
	values := url.Values{}
	values.Set("action", "query")
	values.Set("format", "json")
	values.Set("generator", "search")
	values.Set("gsrsearch", query)
	values.Set("gsrnamespace", "6")
	values.Set("gsrlimit", "12")
	values.Set("prop", "imageinfo")
	values.Set("iiprop", "url|size|mime|extmetadata")
	values.Set("iiurlwidth", "2560")
	request, err := http.NewRequest(http.MethodGet, commonsAPI+"?"+values.Encode(), nil)
	if err != nil {
		return commonsCandidate{}, err
	}
	request.Header.Set("User-Agent", "evercare-journey-backend hot-place collector/1.0")
	response, err := client.Do(request)
	if err != nil {
		return commonsCandidate{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		status := response.StatusCode
		response.Body.Close()
		return commonsCandidate{}, fmt.Errorf("Commons API returned HTTP %d", status)
	}
	var decoded commonsResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(&decoded); err != nil {
		response.Body.Close()
		return commonsCandidate{}, err
	}
	response.Body.Close()
	var candidates []commonsCandidate
	for _, page := range decoded.Query.Pages {
		if len(page.Info) == 0 {
			continue
		}
		info := page.Info[0]
		if info.URL == "" || info.Width < 1200 || info.Height < 600 {
			continue
		}
		aspect := float64(info.Width) / float64(info.Height)
		if aspect < 1.25 {
			continue
		}
		candidates = append(candidates, commonsCandidate{
			Title:          page.Title,
			PageURL:        info.DescriptionURL,
			ImageURL:       info.URL,
			ThumbURL:       chooseString(info.ThumbURL, info.URL),
			RedirectURL:    commonsFileRedirectURL(page.Title),
			RedirectAltURL: commonsFileRedirectAltURL(page.Title),
			ProxyURL:       imageProxyURL(chooseString(info.ThumbURL, info.URL)),
			Width:          info.Width,
			Height:         info.Height,
			ThumbWidth:     info.ThumbWidth,
			ThumbHeight:    info.ThumbHeight,
			License:        metadata(info.ExtMetadata, "LicenseShortName"),
			Artist:         metadata(info.ExtMetadata, "Artist"),
			Credit:         metadata(info.ExtMetadata, "Credit"),
		})
	}
	if len(candidates) == 0 {
		return commonsCandidate{}, fmt.Errorf("no sufficiently large landscape image found for %q", query)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		iScore := imageScore(candidates[i])
		jScore := imageScore(candidates[j])
		return iScore > jScore
	})
	selected := candidates[0]
	if len(candidates) > 1 {
		selected.Alternatives = append([]commonsCandidate(nil), candidates[1:]...)
	}
	return selected, nil
}

func imageScore(candidate commonsCandidate) float64 {
	aspect := float64(candidate.Width) / float64(candidate.Height)
	aspectScore := 1.0 - minFloat(absFloat(aspect-(16.0/9.0)), 1.0)
	resolutionScore := minFloat(float64(candidate.Width)/2560.0, 1.0)
	return aspectScore*2 + resolutionScore
}

func metadata(values map[string]struct {
	Value json.RawMessage `json:"value"`
}, key string) string {
	value := values[key].Value
	if len(value) == 0 {
		return ""
	}
	var text string
	if err := json.Unmarshal(value, &text); err == nil {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(string(value))
}

func download(client *http.Client, imageURLs ...string) ([]byte, error) {
	commonsRequestMu.Lock()
	defer commonsRequestMu.Unlock()
	var lastErr error
	for _, imageURL := range imageURLs {
		if strings.TrimSpace(imageURL) == "" {
			continue
		}
		for attempt := 0; attempt < 3; attempt++ {
			if attempt > 0 {
				time.Sleep(time.Duration(attempt*attempt) * time.Second)
			} else {
				time.Sleep(750 * time.Millisecond)
			}
			request, err := http.NewRequest(http.MethodGet, imageURL, nil)
			if err != nil {
				return nil, err
			}
			request.Header.Set("User-Agent", "evercare-journey-backend hot-place collector/1.0")
			request.Header.Set("Referer", "https://commons.wikimedia.org/")
			response, err := client.Do(request)
			if err != nil {
				return nil, err
			}
			if response.StatusCode < 200 || response.StatusCode >= 300 {
				lastErr = fmt.Errorf("image returned HTTP %d", response.StatusCode)
				response.Body.Close()
				if response.StatusCode == http.StatusTooManyRequests {
					continue
				}
				return nil, lastErr
			}
			data, err := io.ReadAll(io.LimitReader(response.Body, 40<<20))
			response.Body.Close()
			if err != nil {
				return nil, err
			}
			return data, nil
		}
	}
	return nil, lastErr
}

func commonsFileRedirectURL(title string) string {
	fileName := strings.TrimPrefix(title, "File:")
	fileName = strings.ReplaceAll(fileName, " ", "_")
	return "https://commons.wikimedia.org/wiki/Special:FilePath/" + url.PathEscape(fileName) + "?width=2560"
}

func commonsFileRedirectAltURL(title string) string {
	fileName := strings.TrimPrefix(title, "File:")
	return "https://commons.wikimedia.org/w/index.php?title=Special:Redirect/file/" + url.QueryEscape(fileName) + "&width=2560"
}

func imageProxyURL(imageURL string) string {
	values := url.Values{}
	imageURL = strings.TrimPrefix(imageURL, "https://")
	imageURL = strings.TrimPrefix(imageURL, "http://")
	values.Set("url", imageURL)
	values.Set("w", "2560")
	values.Set("output", "jpg")
	return "https://images.weserv.nl/?" + values.Encode()
}

func commonsSearchURL(query string) string {
	return "https://commons.wikimedia.org/w/index.php?search=" + url.QueryEscape(query)
}

func validate(outDir string) error {
	var hotPlaces []hotPlace
	var places []placeCatalogItem
	var sources []sourceRecord
	if err := readJSON(filepath.Join(outDir, "hot_places.json"), &hotPlaces); err != nil {
		return err
	}
	if err := readJSON(filepath.Join(outDir, "place_catalog.json"), &places); err != nil {
		return err
	}
	if err := readJSON(filepath.Join(outDir, "image_sources.json"), &sources); err != nil {
		return err
	}
	if len(hotPlaces) != len(attractions) || len(places) != len(attractions) || len(sources) != len(attractions) {
		return fmt.Errorf("expected %d records in each JSON file, got hot_places=%d places=%d sources=%d", len(attractions), len(hotPlaces), len(places), len(sources))
	}

	placeIDs := make(map[string]struct{}, len(places))
	providerIDs := make(map[string]struct{}, len(places))
	for index, place := range places {
		if _, err := uuid.Parse(place.PlaceIdentity); err != nil {
			return fmt.Errorf("place_catalog[%d] has invalid PlaceIdentity %q", index, place.PlaceIdentity)
		}
		if place.ProviderName != define.PlaceProviderNameDefault {
			return fmt.Errorf("place_catalog[%d] ProviderName is %q, expected %q", index, place.ProviderName, define.PlaceProviderNameDefault)
		}
		if providerID := strings.TrimSpace(place.ProviderPlaceID); providerID == "" || len(providerID) > 64 {
			return fmt.Errorf("place_catalog[%d] has an invalid ProviderPlaceID", index)
		} else if _, exists := providerIDs[providerID]; exists {
			return fmt.Errorf("place_catalog[%d] repeats ProviderPlaceID %q", index, providerID)
		} else {
			providerIDs[providerID] = struct{}{}
		}
		if strings.TrimSpace(place.Name) == "" || strings.TrimSpace(place.CategoryCode) == "" || strings.TrimSpace(place.CategoryName) == "" {
			return fmt.Errorf("place_catalog[%d] has incomplete Amap POI metadata", index)
		}
		expectedCategory, exists := curatedCategoryNames[attractions[index].Slug]
		if !exists {
			return fmt.Errorf("missing curated category for attraction %q", attractions[index].Slug)
		}
		if place.CategoryName != expectedCategory {
			return fmt.Errorf("place_catalog[%d] CategoryName is %q, expected curated category %q", index, place.CategoryName, expectedCategory)
		}
		if err := validateCuratedCategoryName(place.CategoryName); err != nil {
			return fmt.Errorf("place_catalog[%d] has invalid CategoryName: %w", index, err)
		}
		if math.IsNaN(place.Longitude) || math.IsNaN(place.Latitude) || place.Longitude < -180 || place.Longitude > 180 || place.Latitude < -90 || place.Latitude > 90 {
			return fmt.Errorf("place_catalog[%d] has invalid Amap coordinates", index)
		}
		placeIDs[place.PlaceIdentity] = struct{}{}
	}

	maxDetailBytes := 0
	maxDetailRunes := 0
	for index, item := range hotPlaces {
		if item.HotPlaceUniqueID != uint32(index+1) {
			return fmt.Errorf("hot_places[%d] has unexpected HotPlaceUniqueID %d", index, item.HotPlaceUniqueID)
		}
		for fieldName, value := range map[string]string{
			"HotPlaceIdentity": item.HotPlaceIdentity,
			"PlaceImageItemID": item.PlaceImageItemID,
			"PlaceIdentity":    item.PlaceIdentity,
		} {
			parsed, err := uuid.Parse(value)
			if err != nil || parsed == uuid.Nil {
				return fmt.Errorf("hot_places[%d] has invalid %s %q", index, fieldName, value)
			}
		}
		if _, found := placeIDs[item.PlaceIdentity]; !found {
			return fmt.Errorf("hot_places[%d] references missing PlaceIdentity %q", index, item.PlaceIdentity)
		}
		if item.RecommendTitle == "" || item.RecommandDetail == "" {
			return fmt.Errorf("hot_places[%d] has empty recommendation content", index)
		}
		detailRunes := utf8.RuneCountInString(item.RecommandDetail)
		if detailRunes < 700 {
			return fmt.Errorf("hot_places[%d] RecommandDetail is shorter than the 700-character content target", index)
		}
		if detailRunes > 2048 {
			return fmt.Errorf("hot_places[%d] RecommandDetail exceeds the 2048-character storage limit", index)
		}
		if len(item.RecommandDetail) > maxDetailBytes {
			maxDetailBytes = len(item.RecommandDetail)
		}
		if runeCount := utf8.RuneCountInString(item.RecommandDetail); runeCount > maxDetailRunes {
			maxDetailRunes = runeCount
		}
	}

	for index, source := range sources {
		if source.Slug == "" || source.ImageFile == "" {
			return fmt.Errorf("image_sources[%d] has an empty slug or image file", index)
		}
		imagePath := filepath.Join(outDir, filepath.FromSlash(source.ImageFile))
		file, err := os.Open(imagePath)
		if err != nil {
			return fmt.Errorf("image_sources[%d] missing %s: %w", index, source.ImageFile, err)
		}
		config, format, decodeErr := image.DecodeConfig(file)
		file.Close()
		if decodeErr != nil {
			return fmt.Errorf("image_sources[%d] cannot decode %s: %w", index, source.ImageFile, decodeErr)
		}
		if format != "jpeg" || config.Width != imageWidth || config.Height != imageHeight {
			return fmt.Errorf("image_sources[%d] has format=%s size=%dx%d, expected jpeg %dx%d", index, format, config.Width, config.Height, imageWidth, imageHeight)
		}
	}
	fmt.Printf("description limits: max_bytes=%d max_characters=%d\n", maxDetailBytes, maxDetailRunes)
	return nil
}

func readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func normalizeImage(data []byte) ([]byte, error) {
	source, _, err := image.Decode(strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}
	sourceBounds := source.Bounds()
	targetAspect := float64(imageWidth) / float64(imageHeight)
	sourceAspect := float64(sourceBounds.Dx()) / float64(sourceBounds.Dy())
	cropWidth := sourceBounds.Dx()
	cropHeight := sourceBounds.Dy()
	if sourceAspect > targetAspect {
		cropWidth = int(float64(cropHeight) * targetAspect)
	} else {
		cropHeight = int(float64(cropWidth) / targetAspect)
	}
	left := sourceBounds.Min.X + (sourceBounds.Dx()-cropWidth)/2
	top := sourceBounds.Min.Y + (sourceBounds.Dy()-cropHeight)/2
	crop := image.NewRGBA(image.Rect(0, 0, cropWidth, cropHeight))
	xdraw.CatmullRom.Scale(crop, crop.Bounds(), source, image.Rect(left, top, left+cropWidth, top+cropHeight), draw.Src, nil)
	destination := image.NewRGBA(image.Rect(0, 0, imageWidth, imageHeight))
	xdraw.CatmullRom.Scale(destination, destination.Bounds(), crop, crop.Bounds(), draw.Src, nil)
	file, err := os.CreateTemp("", "hotplace-image-*.jpg")
	if err != nil {
		return nil, err
	}
	defer os.Remove(file.Name())
	if err := jpeg.Encode(file, destination, &jpeg.Options{Quality: imageQuality}); err != nil {
		file.Close()
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	return os.ReadFile(file.Name())
}

func writeJSON(path string, value any) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		file.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close %s: %w", path, err)
	}
	return nil
}

func joinErrors(values []error) string {
	parts := make([]string, len(values))
	for index, err := range values {
		parts[index] = err.Error()
	}
	return strings.Join(parts, "\n- ")
}

func chooseString(first string, fallback string) string {
	if strings.TrimSpace(first) != "" {
		return first
	}
	return fallback
}

func minFloat(first float64, second float64) float64 {
	if first < second {
		return first
	}
	return second
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func validateCuratedCategoryName(value string) error {
	if strings.Contains(value, "|") {
		return errors.New("must not contain the provider multi-category separator |")
	}
	labels := strings.Split(value, ";")
	if len(labels) != 3 {
		return fmt.Errorf("must contain exactly 3 semicolon-delimited labels, got %d", len(labels))
	}
	providerLabels := map[string]struct{}{
		"风景名胜": {}, "风景名胜相关": {}, "国家级景点": {}, "省级景点": {},
		"地名地址信息": {}, "自然地名": {}, "热点地名": {},
		"生活服务": {}, "生活服务场所": {}, "科教文化服务": {},
	}
	seen := make(map[string]struct{}, len(labels))
	for _, label := range labels {
		if label == "" || label != strings.TrimSpace(label) {
			return errors.New("labels must be non-empty and must not have surrounding whitespace")
		}
		if utf8.RuneCountInString(label) > 16 {
			return fmt.Errorf("label %q exceeds 16 characters", label)
		}
		if _, exists := seen[label]; exists {
			return fmt.Errorf("label %q is repeated", label)
		}
		if _, isProviderLabel := providerLabels[label]; isProviderLabel {
			return fmt.Errorf("label %q is an Amap provider hierarchy label", label)
		}
		seen[label] = struct{}{}
	}
	return nil
}
