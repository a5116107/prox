"""Idiom Solitaire (成语接龙) - Turn-based idiom chain game for QQ groups."""

import time, re, random
from .base import GamePlugin, GameContext, GameResponse

IDIOM_DB = {
    "一心一意": "yī xīn yī yì",
    "意气风发": "yì qì fēng fā",
    "发愤图强": "fā fèn tú qiáng",
    "强词夺理": "qiǎng cí duó lǐ",
    "理直气壮": "lǐ zhí qì zhuàng",
    "壮志凌云": "zhuàng zhì líng yún",
    "云淡风轻": "yún dàn fēng qīng",
    "轻车熟路": "qīng chē shú lù",
    "路人皆知": "lù rén jiē zhī",
    "知书达理": "zhī shū dá lǐ",
    "理屈词穷": "lǐ qū cí qióng",
    "穷途末路": "qióng tú mò lù",
    "大公无私": "dà gōng wú sī",
    "私心杂念": "sī xīn zá niàn",
    "念念不忘": "niàn niàn bú wàng",
    "忘恩负义": "wàng ēn fù yì",
    "义不容辞": "yì bù róng cí",
    "辞旧迎新": "cí jiù yíng xīn",
    "新陈代谢": "xīn chén dài xiè",
    "谢天谢地": "xiè tiān xiè dì",
    "地大物博": "dì dà wù bó",
    "博学多才": "bó xué duō cái",
    "才华横溢": "cái huá héng yì",
    "龙马精神": "lóng mǎ jīng shén",
    "神采奕奕": "shén cǎi yì yì",
    "前所未有": "qián suǒ wèi yǒu",
    "有目共睹": "yǒu mù gòng dǔ",
    "睹物思人": "dǔ wù sī rén",
    "人山人海": "rén shān rén hǎi",
    "海阔天空": "hǎi kuò tiān kōng",
    "空前绝后": "kōng qián jué hòu",
    "后来居上": "hòu lái jū shàng",
    "上下其手": "shàng xià qí shǒu",
    "手忙脚乱": "shǒu máng jiǎo luàn",
    "乱七八糟": "luàn qī bā zāo",
    "糟糠之妻": "zāo kāng zhī qī",
    "马到成功": "mǎ dào chéng gōng",
    "功成名就": "gōng chéng míng jiù",
    "就地取材": "jiù dì qǔ cái",
    "材大难用": "cái dà nán yòng",
    "用心良苦": "yòng xīn liáng kǔ",
    "苦尽甘来": "kǔ jìn gān lái",
    # Extended dictionary
    "来之不易": "lái zhī bú yì",
    "易如反掌": "yì rú fǎn zhǎng",
    "掌上明珠": "zhǎng shàng míng zhū",
    "珠光宝气": "zhū guāng bǎo qì",
    "气象万千": "qì xiàng wàn qiān",
    "千方百计": "qiān fāng bǎi jì",
    "计日程功": "jì rì chéng gōng",
    "功德无量": "gōng dé wú liàng",
    "量力而行": "liàng lì ér xíng",
    "行云流水": "xíng yún liú shuǐ",
    "水落石出": "shuǐ luò shí chū",
    "出人头地": "chū rén tóu dì",
    "地久天长": "dì jiǔ tiān cháng",
    "长驱直入": "cháng qū zhí rù",
    "入木三分": "rù mù sān fēn",
    "分秒必争": "fēn miǎo bì zhēng",
    "争先恐后": "zhēng xiān kǒng hòu",
    "后顾之忧": "hòu gù zhī yōu",
    "忧心忡忡": "yōu xīn chōng chōng",
    "重蹈覆辙": "chóng dǎo fù zhé",
    "三心二意": "sān xīn èr yì",
    "意犹未尽": "yì yóu wèi jìn",
    "尽善尽美": "jìn shàn jìn měi",
    "美不胜收": "měi bù shèng shōu",
    "收放自如": "shōu fàng zì rú",
    "如鱼得水": "rú yú dé shuǐ",
    "水深火热": "shuǐ shēn huǒ rè",
    "热火朝天": "rè huǒ cháo tiān",
    "天长日久": "tiān cháng rì jiǔ",
    "久负盛名": "jiǔ fù shèng míng",
    "名副其实": "míng fù qí shí",
    "实事求是": "shí shì qiú shì",
    "是非曲直": "shì fēi qū zhí",
    "直言不讳": "zhí yán bù huì",
    "风调雨顺": "fēng tiáo yǔ shùn",
    "顺其自然": "shùn qí zì rán",
    "然后知足": "rán hòu zhī zú",
    "足智多谋": "zú zhì duō móu",
    "谋事在人": "móu shì zài rén",
    "人才辈出": "rén cái bèi chū",
    "出类拔萃": "chū lèi bá cuì",
    "萃然成风": "cuì rán chéng fēng",
    "风华正茂": "fēng huá zhèng mào",
    "茂林修竹": "mào lín xiū zhú",
    "竹报平安": "zhú bào píng ān",
    "安居乐业": "ān jū lè yè",
    "业精于勤": "yè jīng yú qín",
    "勤能补拙": "qín néng bǔ zhuō",
    "心旷神怡": "xīn kuàng shén yí",
    "怡然自得": "yí rán zì dé",
    "得心应手": "dé xīn yìng shǒu",
    "手到擒来": "shǒu dào qín lái",
    "来日方长": "lái rì fāng cháng",
    "长年累月": "cháng nián lěi yuè",
    "月明星稀": "yuè míng xīng xī",
    "稀世之宝": "xī shì zhī bǎo",
    "宝刀未老": "bǎo dāo wèi lǎo",
    "老当益壮": "lǎo dāng yì zhuàng",
    "壮士断腕": "zhuàng shì duàn wàn",
    "万紫千红": "wàn zǐ qiān hóng",
    "红颜薄命": "hóng yán bó mìng",
    "命中注定": "mìng zhōng zhù dìng",
    "定国安邦": "dìng guó ān bāng",
    "邦国栋梁": "bāng guó dòng liáng",
    "心想事成": "xīn xiǎng shì chéng",
    "成竹在胸": "chéng zhú zài xiōng",
    "胸有成竹": "xiōng yǒu chéng zhú",
    "竹篮打水": "zhú lán dǎ shuǐ",
    "水到渠成": "shuǐ dào qú chéng",
    "成事不足": "chéng shì bù zú",
    "足不出户": "zú bù chū hù",
    "户枢不蠹": "hù shū bù dù",
    "独当一面": "dú dāng yī miàn",
    "面目全非": "miàn mù quán fēi",
    "非同小可": "fēi tóng xiǎo kě",
    "可歌可泣": "kě gē kě qì",
    "泣不成声": "qì bù chéng shēng",
    "声色俱厉": "shēng sè jù lì",
    "厉兵秣马": "lì bīng mò mǎ",
    "马不停蹄": "mǎ bù tíng tí",
    "天衣无缝": "tiān yī wú fèng",
    "缝缝补补": "féng féng bǔ bǔ",
    "步步为营": "bù bù wéi yíng",
    "营私舞弊": "yíng sī wǔ bì",
    "弊绝风清": "bì jué fēng qīng",
    "清风明月": "qīng fēng míng yuè",
    "月下花前": "yuè xià huā qián",
    "前程万里": "qián chéng wàn lǐ",
    "里应外合": "lǐ yìng wài hé",
    "合情合理": "hé qíng hé lǐ",
    "理所当然": "lǐ suǒ dāng rán",
    "然而不同": "rán ér bù tóng",
    "同甘共苦": "tóng gān gòng kǔ",
    "苦中作乐": "kǔ zhōng zuò lè",
    "乐极生悲": "lè jí shēng bēi",
    "悲欢离合": "bēi huān lí hé",
    "合二为一": "hé èr wéi yī",
    "一鸣惊人": "yī míng jīng rén",
    "人杰地灵": "rén jié dì líng",
    "灵机一动": "líng jī yī dòng",
    "动人心弦": "dòng rén xīn xián",
    "弦外之音": "xián wài zhī yīn",
    "音容笑貌": "yīn róng xiào mào",
    "貌合神离": "mào hé shén lí",
    "离经叛道": "lí jīng pàn dào",
    "道听途说": "dào tīng tú shuō",
    "说一不二": "shuō yī bù èr",
    "二话不说": "èr huà bù shuō",
    "四面楚歌": "sì miàn chǔ gē",
    "歌舞升平": "gē wǔ shēng píng",
    "平分秋色": "píng fēn qiū sè",
    "色厉内荏": "sè lì nèi rěn",
    "日新月异": "rì xīn yuè yì",
    "异口同声": "yì kǒu tóng shēng",
    "声东击西": "shēng dōng jī xī",
    "西装革履": "xī zhuāng gé lǚ",
    "花好月圆": "huā hǎo yuè yuán",
    "圆满成功": "yuán mǎn chéng gōng",
    "功亏一篑": "gōng kuī yī kuì",
    "愚公移山": "yú gōng yí shān",
    "山清水秀": "shān qīng shuǐ xiù",
    "秀外慧中": "xiù wài huì zhōng",
    "中流砥柱": "zhōng liú dǐ zhù",
    "柱石之臣": "zhù shí zhī chén",
}
IDIOMS = [k for k, v in IDIOM_DB.items() if v is not None]


def first_char(w):
    for ch in w:
        if "一" <= ch <= "鿿":
            return ch
    return w[0] if w else ""


def last_char(w):
    for ch in reversed(w):
        if "一" <= ch <= "鿿":
            return ch
    return w[-1] if w else ""


def valid_idiom(w):
    return w in IDIOM_DB and IDIOM_DB[w] is not None


def idiom_matches(last_word, candidate):
    return first_char(candidate) == last_char(last_word)


class IdiomGame(GamePlugin):
    name = "duel_idiom"
    display_name = "成语接龙"
    description = "轮流接成语，接不上或重复即输"
    tier = "p2p"
    triggers = ["成语接龙", "idiom", "接龙", "成接"]
    group_required = True
    default_config = {
        "enabled": True,
        "reward_quota": 50000,
        "max_rounds": 20,
        "turn_timeout_sec": 60,
        "max_per_user_day": 30,
        "cooldown_seconds": 0,
        "budget_pool": "activity",
    }

    def __init__(self):
        super().__init__()
        self._games = {}

    def handle(self, ctx, sm, budget, escrow):
        t = ctx.text.strip()
        gid = ctx.group_id
        uid = ctx.user_id
        un = ctx.username
        state = self._games.get(gid)

        if t.lower().startswith("成语接龙") and "@" in t:
            parts = t.split()
            opponent = None
            for p in parts[1:]:
                if p.startswith("@"):
                    opponent = p[1:]
                    break
            if not opponent:
                return GameResponse.quick(f"@{un} 格式: `成语接龙 @对手`")

            starter = random.choice(IDIOMS)
            self._games[gid] = {
                "player1": uid,
                "player1_napi": int(ctx.new_api_user_id or 0),
                "player1_name": un,
                "player2": None,
                "player2_napi": 0,
                "player2_name": opponent,
                "current_player": uid,
                "last_word": starter,
                "used": {starter},
                "rounds": 0,
                "started_at": time.time(),
            }
            msg = (
                f"@{un} 🎯 发起成语接龙！对手: @{opponent}\n"
                f"📝 起始成语: **{starter}**\n"
                f"⏰ 每轮 {self.config['turn_timeout_sec']}秒\n"
                f"💡 直接回复成语即可（首字需接上一个成语的尾字）"
            )
            return GameResponse(reply=msg, event="idiom_init")

        if state:
            if state["player2"] is None and un == state.get("player2_name"):
                state["player2"] = uid
                state["player2_napi"] = int(ctx.new_api_user_id or 0)

            if uid not in (state["player1"], state["player2"]):
                return GameResponse.quick(f"@{un} 你不在当前对局中~")
            if uid != state["current_player"]:
                return GameResponse.quick(f"@{un} 还没轮到你，等对手接招吧")

            candidate = t.strip()

            if candidate in state["used"]:
                return self._end_game(budget, ctx, state, uid, "重复了")

            if not valid_idiom(candidate):
                return self._end_game(
                    budget, ctx, state, uid, f'"{candidate}" 不是有效成语'
                )

            if not idiom_matches(state["last_word"], candidate):
                return self._end_game(
                    budget,
                    ctx,
                    state,
                    uid,
                    f'首字"{first_char(candidate)}" ≠ 上尾"{last_char(state["last_word"])}"',
                )

            state["used"].add(candidate)
            state["last_word"] = candidate
            state["rounds"] += 1
            self.record_play(uid)

            if state["rounds"] >= self.config["max_rounds"]:
                self._games.pop(gid, None)
                return GameResponse.quick(f"🎉 已达 {state['rounds']} 回合上限，平局！")

            state["current_player"] = (
                state["player2"] if uid == state["player1"] else state["player1"]
            )
            next_name = (
                state["player2_name"]
                if uid == state["player1"]
                else state["player1_name"]
            )

            msg = (
                f"✅ @{un} 接: **{candidate}**\n"
                f'⏩ 轮到 @{next_name}，需以 "{last_char(candidate)}" 开头\n'
                f"📊 回合: {state['rounds']}/{self.config['max_rounds']}"
            )
            return GameResponse(reply=msg, event="idiom_turn")

        return GameResponse.quick(f"@{un} 格式: `成语接龙 @对手`")

    def _end_game(self, budget, ctx, state, loser_id, reason):
        gid = ctx.group_id
        winner_id = (
            state["player2"] if loser_id == state["player1"] else state["player1"]
        )
        winner_napi = int(
            (
                state.get("player2_napi")
                if loser_id == state["player1"]
                else state.get("player1_napi")
            )
            or 0
        )
        winner_name = (
            state["player2_name"]
            if loser_id == state["player1"]
            else state["player1_name"]
        )
        loser_name = (
            state["player1_name"]
            if loser_id == state["player1"]
            else state["player2_name"]
        )
        self._games.pop(gid, None)

        reward = self.config["reward_quota"]
        msg = (
            f"💀 **成语接龙结束**\n"
            f"原因: {reason}\n"
            f"😢 @{loser_name} 败北\n"
            f"🏆 恭喜 @{winner_name} 获胜！+${budget.quota_to_usd(reward):.2f}"
        )

        actions = []
        if winner_napi:
            actions.append(
                {
                    "type": "reward.grant.small",
                    "target_type": "user",
                    "user_id": winner_napi,
                    "quota_amount": reward,
                    "budget_pool": self.config["budget_pool"],
                    "reason": "idiom_win",
                }
            )

        return GameResponse(reply=msg, actions=actions, event="idiom_end")
