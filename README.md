# 🎬 youtube-gir

یک رابط حرفه‌ای، سریع و تعاملی برای `yt-dlp` که دانلود صوت و ویدئو را با چند preset ساده می‌کند. اگر فقط لینک را بدهید، دانلود با کیفیت **عالی** شروع می‌شود؛ اگر هیچ آرگومانی ندهید، منوی تعاملی باز خواهد شد.

```bash
youtube-gir 'https://youtu.be/px_WwBtcoTI'

# نام سازگار با نمونهٔ اولیهٔ پروژه نیز کار می‌کند:
youtube-git 'https://youtu.be/px_WwBtcoTI'
```

> نام اصلی برنامه و ریپو `youtube-gir` است. نصب‌کننده برای سازگاری، فرمان `youtube-git` را نیز به‌صورت alias می‌سازد.

## ✨ امکانات

- چهار کیفیت `excellent`، `normal`، `good` و `low` برای صوت و ویدئو
- خروجی صوتی M4A/AAC و خروجی ویدئویی MP4 با Fast Start
- اجرای سریع فقط با `youtube-gir LINK` و منوی تعاملی با `youtube-gir`
- تنظیم مستقل پوشهٔ پیش‌فرض صوت و ویدئو، یا `--output` برای هر اجرا
- استخراج کوکی از Chrome (قابل تغییر به Firefox و مرورگرهای پشتیبانی‌شده)
- حذف خودکار بخش‌های `sponsor` و `selfpromo` با SponsorBlock
- فایل archive برای ردکردن ویدئوهایی که قبلاً دانلود شده‌اند
- کش پایدار yt-dlp، retry خودکار و نمایش درصد، سرعت، ETA و خطای کامل
- metadata، chapter و thumbnail قابل جاسازی
- بررسی نسخهٔ برنامه و yt-dlp مستقیماً از GitHub
- آپدیت هستهٔ رسمی yt-dlp و self-update برنامه
- کانفیگ امن `key=value` که به‌عنوان کد shell اجرا نمی‌شود
- سازگار با macOS و Linux روی Bash 3.2+

## 📦 نصب

### نصب یک‌خطی مستقیم از GitHub

```bash
curl -fsSL https://raw.githubusercontent.com/WhoisGray/youtube-gir/main/install.sh | bash -s -- --yes
```

این فرمان حتی بدون clone کردن ریپو کار می‌کند. نصب‌کننده آخرین GitHub Release را پیدا می‌کند و اگر هنوز Release منتشر نشده باشد، نسخهٔ شاخهٔ `main` را می‌گیرد.

> خود `youtube-gir` در `~/.local/bin` نصب می‌شود و به `sudo` نیاز ندارد. فقط نصب اولیهٔ Homebrew ممکن است برای ساخت مسیرهای سیستمی، از طریق نصب‌کنندهٔ رسمی خودش رمز مدیر بخواهد. کل فرمان بالا را با `sudo` اجرا نکنید؛ در غیر این صورت فایل‌ها ممکن است وارد Home کاربر root شوند.

اگر Homebrew هنوز نصب نیست و سیستم برای آن دسترسی مدیر می‌خواهد، می‌توانید فقط مجوز sudo را پیشاپیش تأیید کنید و سپس نصب را با کاربر عادی ادامه دهید:

```bash
sudo -v && curl -fsSL https://raw.githubusercontent.com/WhoisGray/youtube-gir/main/install.sh | bash -s -- --yes
```

### نصب با clone

```bash
git clone https://github.com/WhoisGray/youtube-gir.git
cd youtube-gir
bash install.sh
```

نصب‌کننده موارد زیر را بررسی یا نصب می‌کند:

1. Homebrew با نصب‌کنندهٔ رسمی `brew.sh`
2. Node.js و FFmpeg از Homebrew
3. Deno با نصب‌کنندهٔ رسمی `deno.land`
4. باینری release رسمی yt-dlp از `yt-dlp/yt-dlp` با بررسی SHA-256
5. دو فرمان `youtube-gir` و `youtube-git` در `~/.local/bin`

در اجرای مجدد، نصب‌کننده این بررسی‌ها را انجام می‌دهد:

- اجرای `brew update` و ارتقای Node.js و FFmpeg فقط در صورت وجود نسخهٔ جدید
- بررسی و ارتقای Deno با updater مربوط به روش نصب آن
- مقایسهٔ نسخهٔ محلی yt-dlp با آخرین GitHub Release؛ دانلود مجدد فقط در صورت نیاز
- مقایسهٔ نسخهٔ محلی youtube-gir با نسخهٔ GitHub؛ نصب فقط وقتی نسخه تغییر کرده باشد
- بازسازی alias `youtube-git` در صورت حذف‌شدن آن

برای نصب بدون سؤال در حالت clone:

```bash
bash install.sh --yes
```

اگر پیش‌نیازها را قبلاً نصب کرده‌اید:

```bash
bash install.sh --no-deps
```

برای نصب مجدد اجباری، حتی اگر شماره نسخه یکسان باشد:

```bash
bash install.sh --force
```

اگر `~/.local/bin` در `PATH` نیست، خطی را که نصب‌کننده نمایش می‌دهد به `~/.zshrc` یا `~/.bashrc` اضافه کنید. مسیر نصب را نیز می‌توان عوض کرد:

```bash
YOUTUBE_GIR_BIN_DIR="$HOME/bin" bash install.sh --no-deps
```

## 🧰 نصب دستی پیش‌نیازها

نصب‌کنندهٔ یک‌خطی همهٔ این موارد را خودش بررسی می‌کند. اگر ترجیح می‌دهید آن‌ها را دستی نصب کنید، مراحل زیر را انجام دهید.

### 1. Homebrew

روی macOS ابتدا Command Line Tools لازم است:

```bash
xcode-select --install
```

سپس Homebrew را با دستور رسمی نصب کنید:

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
```

همین نصب‌کننده روی توزیع‌های پشتیبانی‌شدهٔ Linux نیز کار می‌کند. در Linux ممکن است ابتدا ابزارهایی مانند `build-essential` و `curl` را با package manager سیستم نصب کنید.

### 2. Node.js و FFmpeg

```bash
brew install node ffmpeg
```

### 3. Deno

```bash
curl -fsSL https://deno.land/install.sh | sh
export PATH="$HOME/.deno/bin:$PATH"
```

### 4. yt-dlp رسمی

macOS:

```bash
mkdir -p "$HOME/.local/bin"
curl -fL https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_macos -o "$HOME/.local/bin/yt-dlp"
chmod 0755 "$HOME/.local/bin/yt-dlp"
```

Linux روی x86_64:

```bash
mkdir -p "$HOME/.local/bin"
curl -fL https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_linux -o "$HOME/.local/bin/yt-dlp"
chmod 0755 "$HOME/.local/bin/yt-dlp"
```

برای Linux ARM64 نام فایل `yt-dlp_linux_aarch64` است. نصب‌کنندهٔ خود پروژه معماری را خودکار تشخیص می‌دهد و checksum رسمی `SHA2-256SUMS` را نیز کنترل می‌کند، بنابراین روش پیشنهادی همان نصب یک‌خطی است.

### 5. بررسی سیستم

```bash
export PATH="$HOME/.local/bin:$HOME/.deno/bin:$PATH"
youtube-gir doctor
node --version
deno --version
yt-dlp --version
ffmpeg -version
```

## 🚀 استفاده

دانلود پیش‌فرض (ویدئو + صدا، 1080p):

```bash
youtube-gir LINK
```

حالت تعاملی:

```bash
youtube-gir
```

صدا با کیفیت مشخص:

```bash
youtube-gir --audio --quality good LINK
```

ویدئو با حذف sponsor و خروجی سفارشی:

```bash
youtube-gir --video --quality excellent \
  --sponsorblock \
  --output "$HOME/Videos" \
  LINK
```

playlist با ردکردن موارد دانلودشده:

```bash
youtube-gir --playlist --archive PLAYLIST_LINK
```

حالت سریع، پردازش thumbnail/chapter را حذف و fragmentها را هم‌زمان دریافت می‌کند:

```bash
youtube-gir --fast LINK
```

برای دیدن فرمان نهایی بدون دانلود:

```bash
youtube-gir --dry-run --no-cookies LINK
```

راهنمای کامل:

```bash
youtube-gir --help
```

## 🎚️ جدول presetها

| نام | ویدئو | صوت AAC |
|---|---:|---:|
| `excellent` / عالی | تا 1080p | 192 kbps |
| `normal` / معمولی | تا 720p | 160 kbps |
| `good` / خوب | تا 480p | 128 kbps |
| `low` / پایین | تا 360p | 96 kbps |

انتخاب format چند fallback دارد؛ اگر H.264/AAC در کیفیت خواسته‌شده وجود نداشته باشد، yt-dlp بهترین فرمت سازگار بعدی را انتخاب می‌کند و FFmpeg خروجی نهایی را آماده می‌سازد.

## ⚙️ تنظیمات

ویرایش تعاملی تنظیمات:

```bash
youtube-gir config
```

مسیر پیش‌فرض کانفیگ:

```text
~/.config/youtube-gir/config
```

نمونهٔ کامل در [`config.example`](config.example) قرار دارد:

```ini
default_media=video
default_quality=excellent
audio_dir=
video_dir=
cookies_browser=chrome
use_cookies=true
sponsorblock=true
use_archive=false
archive_file=~/.local/share/youtube-gir/archive.txt
youtube_player_client=mweb
embed_thumbnail=true
embed_metadata=true
```

اگر `audio_dir` یا `video_dir` خالی باشد، خروجی در همان پوشه‌ای ذخیره می‌شود که دستور را اجرا کرده‌اید. اولویت انتخاب مسیر چنین است:

```text
--output  ←  مسیر نوع رسانه در کانفیگ  ←  پوشهٔ جاری
```

مسیرهای استاندارد دیگر:

- کش: `~/.cache/youtube-gir/yt-dlp`
- داده و آرشیو: `~/.local/share/youtube-gir/archive.txt`
- متغیرهای `XDG_CONFIG_HOME`، `XDG_CACHE_HOME` و `XDG_DATA_HOME` پشتیبانی می‌شوند.

## 🛡️ SponsorBlock و archive

فعال‌سازی فقط برای یک اجرا:

```bash
youtube-gir --sponsorblock LINK
youtube-gir --archive LINK
youtube-gir --archive-file "$HOME/.downloads.archive" LINK
```

برای فعال‌بودن دائمی، `sponsorblock=true` یا `use_archive=true` را در کانفیگ قرار دهید. با `--no-sponsorblock` می‌توانید تنظیم دائمی SponsorBlock را برای یک اجرا خاموش کنید.

`--archive` فقط شناسهٔ دانلود موفق را ثبت می‌کند و جلوی دانلود دوباره را می‌گیرد؛ فایل رسانه را حذف نمی‌کند.

## 🔄 بررسی و آپدیت

```bash
# مقایسهٔ نسخهٔ نصب‌شده با آخرین GitHub Release برنامه و yt-dlp
youtube-gir check-update

# فقط هستهٔ رسمی yt-dlp
youtube-gir update-core

# برنامه و سپس هستهٔ yt-dlp
youtube-gir update
```

Self-update برنامه از آخرین Release ریپوی [`WhoisGray/youtube-gir`](https://github.com/WhoisGray/youtube-gir) انجام می‌شود. تا پیش از انتشار اولین Release، `update` بخش برنامه را رد می‌کند و همچنان آپدیت هستهٔ yt-dlp را ادامه می‌دهد؛ `update-core` نیز مستقل کار می‌کند.

## 🩺 رفع اشکال

ابتدا این فرمان را اجرا کنید:

```bash
youtube-gir doctor
```

چند نکتهٔ رایج:

- برای خطای cookie، Chrome را کامل ببندید یا از `--cookies firefox` / `--no-cookies` استفاده کنید.
- اگر YouTube ساختار player را تغییر داد، ابتدا `youtube-gir update-core` را اجرا کنید.
- برنامه stderr و stdout مربوط به yt-dlp/FFmpeg را پنهان نمی‌کند؛ خطای کامل بالای پیام نهایی باقی می‌ماند. برای گزارش تشخیصی گسترده‌تر از `--verbose` استفاده کنید.
- `--no-playlist` پیش‌فرض است تا یک لینک ناخواسته کل playlist را دانلود نکند.
- برای نمایش جزئیات داخلی yt-dlp مستقیماً `youtube-gir --verbose LINK` را اجرا کنید.

## 🧪 توسعه

```bash
bash -n bin/youtube-gir install.sh tests/cli_test.sh tests/installer_test.sh
bash tests/cli_test.sh
bash tests/installer_test.sh
shellcheck bin/youtube-gir install.sh tests/cli_test.sh tests/installer_test.sh
```

GitHub Actions همین سه بررسی را روی push و pull request اجرا می‌کند. برای انتشار نسخه، شمارهٔ `APP_VERSION` در `bin/youtube-gir` را تغییر دهید و tag هم‌نام مانند `v1.0.0` بسازید.

## منابع رسمی

- [Homebrew Installation](https://docs.brew.sh/Installation)
- [Deno Installation](https://docs.deno.com/runtime/getting_started/installation/)
- [Node.js Downloads](https://nodejs.org/en/download)
- [yt-dlp Releases](https://github.com/yt-dlp/yt-dlp/releases/latest)
- [FFmpeg](https://ffmpeg.org/)

## ⚖️ مسئولیت استفاده

فقط محتوایی را دانلود کنید که اجازهٔ دانلود و استفاده از آن را دارید. قوانین سرویس، حق نشر و قوانین محل زندگی شما همچنان برقرارند. این پروژه وابسته یا مورد تأیید YouTube نیست.

## مجوز

[MIT](LICENSE)

---

## ⬇️ نصب مستقیم نهایی

```bash
curl -fsSL https://raw.githubusercontent.com/WhoisGray/youtube-gir/main/install.sh | bash -s -- --yes
```
