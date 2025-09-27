[فارسی](/README.fa_IR.md) | [انگلیسی](/README.md) | [چینی](/README.zh_CN.md) | [اسپانیایی](/README.es_ES.md) | [روسی](/README.ru_RU.md)

<p align="center">
  <picture>
    <img alt="tx-ui" src="./media/tx-ui-dark.png" style="width:512px;height:512px;">
  </picture>
</p>

**یک پنل وب پیشرفته • ساخته شده بر روی هسته Xray**
**این پروژه یک فورک از پنل 3x-ui است.**

[![](https://img.shields.io/github/v/release/AghayeCoder/tx-ui.svg)](https://github.com/AghayeCoder/tx-ui/releases)
[![](https://img.shields.io/github/actions/workflow/status/AghayeCoder/tx-ui/release.yml.svg)](#)
[![GO Version](https://img.shields.io/github/go-mod/go-version/AghayeCoder/tx-ui.svg)](#)
[![Downloads](https://img.shields.io/github/downloads/AghayeCoder/tx-ui/total.svg)](#)
[![License](https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true)](https://www.gnu.org/licenses/gpl-3.0.en.html)

> **سلب مسئولیت:** این پروژه فقط برای یادگیری و ارتباطات شخصی است، لطفاً از آن برای مقاصد غیرقانونی استفاده نکنید، لطفاً
> از آن در محیط عملیاتی (production) استفاده نکنید.

**اگر این پروژه برای شما مفید بود، ممکن است بخواهید به آن یک ستاره بدهید**:star2:

## نصب و ارتقا

```bash
< (curl -Ls https://raw.githubusercontent.com/AghayeCoder/tx-ui/master/install.sh)
```

## گواهی SSL

<details>
  <summary>برای جزئیات گواهی SSL کلیک کنید</summary>

### ACME

برای مدیریت گواهی‌های SSL با استفاده از ACME:

1. اطمینان حاصل کنید که دامنه شما به درستی به سرور متصل شده است.
2. دستور `x-ui` را در ترمینال اجرا کنید، سپس `مدیریت گواهی SSL` را انتخاب کنید.
3. گزینه‌های زیر به شما نمایش داده می‌شود:

    - **دریافت SSL:** دریافت گواهی‌های SSL.
    - **لغو:** لغو گواهی‌های SSL موجود.
    - **تمدید اجباری:** تمدید اجباری گواهی‌های SSL.
    - **نمایش دامنه‌های موجود:** نمایش تمام گواهی‌های دامنه موجود در سرور.
    - **تنظیم مسیرهای گواهی برای پنل:** مشخص کردن گواهی دامنه شما برای استفاده توسط پنل.

### Certbot

برای نصب و استفاده از Certbot:

```sh
apt-get install certbot -y
certbot certonly --standalone --agree-tos --register-unsafely-without-email -d yourdomain.com
certbot renew --dry-run
```

### Cloudflare

اسکریپت مدیریت شامل یک برنامه داخلی برای درخواست گواهی SSL از Cloudflare است. برای استفاده از این اسکریپت جهت درخواست
گواهی، به موارد زیر نیاز دارید:

- ایمیل ثبت‌شده در Cloudflare
- کلید API سراسری (Global API Key) Cloudflare
- نام دامنه باید از طریق Cloudflare به سرور فعلی متصل شده باشد

**چگونه کلید API سراسری Cloudflare را دریافت کنیم:**

1. دستور `x-ui` را در ترمینال اجرا کنید، سپس `گواهی SSL کلودفلر` را انتخاب کنید.
2. از این لینک بازدید کنید: [توکن‌های API کلودفلر](https://dash.cloudflare.com/profile/api-tokens).
3. روی "View Global API Key" کلیک کنید (تصویر زیر را ببینید):
   ![](media/APIKey1.PNG)
4. ممکن است نیاز به احراز هویت مجدد حساب خود داشته باشید. پس از آن، کلید API نمایش داده خواهد شد (تصویر زیر را ببینید):
   ![](media/APIKey2.png)

هنگام استفاده، فقط `نام دامنه`، `ایمیل`، و `کلید API` خود را وارد کنید. نمودار به شرح زیر است:
![](media/DetailEnter.png)


</details>

## نصب و ارتقا دستی

<details>
  <summary>برای جزئیات نصب دستی کلیک کنید</summary>

#### نحوه استفاده

1. برای دانلود آخرین نسخه بسته فشرده به طور مستقیم روی سرور خود، دستور زیر را اجرا کنید:

```bash
ARCH=$(uname -m)
case "${ARCH}" in
  x86_64 | x64 | amd64) XUI_ARCH="amd64" ;; 
  i*86 | x86) XUI_ARCH="386" ;; 
  armv8* | armv8 | arm64 | aarch64) XUI_ARCH="arm64" ;; 
  armv7* | armv7) XUI_ARCH="armv7" ;; 
  armv6* | armv6) XUI_ARCH="armv6" ;; 
  armv5* | armv5) XUI_ARCH="armv5" ;; 
  s390x) echo 's390x' ;; 
  *) XUI_ARCH="amd64" ;; 
esac


wget https://github.com/AghayeCoder/tx-ui/releases/latest/download/x-ui-linux-${XUI_ARCH}.tar.gz
```

2. پس از دانلود بسته فشرده، دستورات زیر را برای نصب یا ارتقا x-ui اجرا کنید:

```bash
ARCH=$(uname -m)
case "${ARCH}" in
  x86_64 | x64 | amd64) XUI_ARCH="amd64" ;; 
  i*86 | x86) XUI_ARCH="386" ;; 
  armv8* | armv8 | arm64 | aarch64) XUI_ARCH="arm64" ;; 
  armv7* | armv7) XUI_ARCH="armv7" ;; 
  armv6* | armv6) XUI_ARCH="armv6" ;; 
  armv5* | armv5) XUI_ARCH="armv5" ;; 
  s390x) echo 's390x' ;; 
  *) XUI_ARCH="amd64" ;; 
esac

cd /root/ 
rm -rf x-ui/ /usr/local/x-ui/ /usr/bin/x-ui
tar zxvf x-ui-linux-${XUI_ARCH}.tar.gz
chmod +x x-ui/x-ui x-ui/bin/xray-linux-* x-ui/x-ui.sh
cp x-ui/x-ui.sh /usr/bin/x-ui
cp -f x-ui/x-ui.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable x-ui
systemctl restart x-ui
```

</details>

## نصب با Docker

<details>
  <summary>برای جزئیات Docker کلیک کنید</summary>

#### نحوه استفاده

1. **نصب Docker:**

   ```sh
   bash <(curl -sSL https://get.docker.com)
   ```

2. **کلون کردن ریپازیتوری پروژه:**

   ```sh
   git clone https://github.com/AghayeCoder/tx-ui.git
   cd tx-ui
   ```

3. **شروع سرویس:**

   ```sh
   docker compose up -d
   ```

فلگ `--pull always` را اضافه کنید تا داکر در صورت کشیده شدن یک ایمیج جدیدتر، کانتینر را به طور خودکار بازسازی کند.
برای اطلاعات بیشتر به https://docs.docker.com/reference/cli/docker/container/run/#pull مراجعه کنید.

**یا**

   ```sh
   docker run -itd \
      -e XRAY_VMESS_AEAD_FORCED=false \
      -v $PWD/db/:/etc/x-ui/ \
      -v $PWD/cert/:/root/cert/ \
      --network=host \
      --restart=unless-stopped \
      --name tx-ui \
      ghcr.io/aghayecoder/tx-ui:latest
   ```

4. **به‌روزرسانی به آخرین نسخه:**

   ```sh
   cd tx-ui
   docker compose down
   docker compose pull tx-ui
   docker compose up -d
   ```

5. **حذف tx-ui از Docker:**

   ```sh
   docker stop tx-ui
   docker rm tx-ui
   cd --
   rm -r tx-ui
   ```

</details>

## تنظیمات Nginx

<details>
  <summary>برای تنظیمات پراکسی معکوس کلیک کنید</summary>

#### پراکسی معکوس Nginx

```nginx
location / {
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header Host $http_host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header Range $http_range;
    proxy_set_header If-Range $http_if_range; 
    proxy_redirect off;
    proxy_pass http://127.0.0.1:2053;
}
```

#### زیرمسیر Nginx

- اطمینان حاصل کنید که "مسیر URI" در تنظیمات پنل `/sub` یکسان باشد.
- `url` در تنظیمات پنل باید با `/` خاتمه یابد.

```nginx
location /sub {
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header Host $http_host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header Range $http_range;
    proxy_set_header If-Range $http_if_range; 
    proxy_redirect off;
    proxy_pass http://127.0.0.1:2053;
}
```

</details>

## سیستم‌عامل‌های پیشنهادی

- Ubuntu 22.04+
- Debian 12+
- CentOS 8+
- OpenEuler 22.03+
- Fedora 36+
- Arch Linux
- Parch Linux
- Manjaro
- Armbian
- AlmaLinux 9.5+
- Rocky Linux 9.5+
- Oracle Linux 8+
- OpenSUSE Tubleweed
- Amazon Linux 2023
- Windows x64

## معماری‌ها و دستگاه‌های پشتیبانی شده

<details>
  <summary>برای جزئیات معماری‌ها و دستگاه‌های پشتیبانی شده کلیک کنید</summary>

پلتفرم ما با طیف متنوعی از معماری‌ها و دستگاه‌ها سازگاری دارد و انعطاف‌پذیری را در محیط‌های مختلف محاسباتی تضمین می‌کند.
معماری‌های کلیدی که ما پشتیبانی می‌کنیم به شرح زیر است:

- **amd64**: این معماری رایج، استاندارد کامپیوترهای شخصی و سرورها است و اکثر سیستم‌عامل‌های مدرن را به راحتی پشتیبانی
  می‌کند.

- **x86 / i386**: این معماری که به طور گسترده در کامپیوترهای رومیزی و لپ‌تاپ‌ها استفاده می‌شود، از پشتیبانی وسیع
  سیستم‌عامل‌ها و برنامه‌های کاربردی متعددی از جمله، اما نه محدود به، سیستم‌های ویندوز، macOS و لینوکس برخوردار است.

- **armv8 / arm64 / aarch64**: این معماری که برای دستگاه‌های موبایل و توکار (embedded) امروزی مانند گوشی‌های هوشمند و
  تبلت‌ها طراحی شده است، در دستگاه‌هایی مانند Raspberry Pi 4، Raspberry Pi 3، Raspberry Pi Zero 2/Zero 2 W، Orange Pi 3
  LTS و غیره به کار رفته است.

- **armv7 / arm / arm32**: این معماری که برای دستگاه‌های موبایل و توکار قدیمی‌تر استفاده می‌شود، همچنان به طور گسترده در
  دستگاه‌هایی مانند Orange Pi Zero LTS، Orange Pi PC Plus، Raspberry Pi 2 و غیره کاربرد دارد.

- **armv6 / arm / arm32**: این معماری که برای دستگاه‌های توکار بسیار قدیمی طراحی شده، با وجود رواج کمتر، هنوز هم در حال
  استفاده است. دستگاه‌هایی مانند Raspberry Pi 1، Raspberry Pi Zero/Zero W از این معماری استفاده می‌کنند.

- **armv5 / arm / arm32**: یک معماری قدیمی‌تر که عمدتاً با سیستم‌های توکار اولیه مرتبط است، امروزه کمتر رایج است اما
  ممکن است هنوز در دستگاه‌های قدیمی مانند نسخه‌های اولیه Raspberry Pi و برخی گوشی‌های هوشمند قدیمی‌تر یافت شود.

- **s390x**: این معماری معمولاً در کامپیوترهای مین‌فریم IBM استفاده می‌شود و عملکرد و قابلیت اطمینان بالایی را برای
  بارهای کاری سازمانی ارائه می‌دهد.

</details>

## زبان‌ها

- عربی
- انگلیسی
- فارسی
- چینی سنتی
- چینی ساده‌شده
- ژاپنی
- روسی
- ویتنامی
- اسپانیایی
- اندونزیایی
- اوکراینی
- ترکی
- پرتغالی (برزیل)

## ویژگی‌ها

- نصب پنل با Force HTTPs
- نظارت بر وضعیت سیستم
- جستجو در میان تمام ورودی‌ها و کلاینت‌ها
- تم تاریک/روشن
- پشتیبانی از چند کاربر و چند پروتکل
- پشتیبانی از پروتکل‌ها، از جمله VMESS، VLESS، Trojan، Shadowsocks، Dokodemo-door، Socks، HTTP، wireguard
- پشتیبانی از پروتکل‌های بومی XTLS، از جمله RPRX-Direct، Vision، REALITY
- آمار ترافیک، محدودیت ترافیک، محدودیت زمان انقضا
- قالب‌های قابل تنظیم پیکربندی Xray
- پشتیبانی از دسترسی به پنل با HTTPS (دامنه و گواهی SSL توسط خودتان ارائه می‌شود)
- پشتیبانی از درخواست گواهی SSL با یک کلیک و تمدید خودکار
- برای موارد پیکربندی پیشرفته‌تر، لطفاً به پنل مراجعه کنید
- رفع مسیرهای API (تنظیمات کاربر با API ایجاد خواهد شد)
- پشتیبانی از تغییر تنظیمات بر اساس موارد مختلف ارائه شده در پنل
- پشتیبانی از صادرات/واردات پایگاه داده از پنل
- به‌روزرسان داخلی برنامه

## تنظیمات پیش‌فرض پنل

<details>
  <summary>برای جزئیات تنظیمات پیش‌فرض کلیک کنید</summary>

### نام کاربری، رمز عبور، پورت و مسیر پایه وب

اگر این تنظیمات را تغییر ندهید، به صورت تصادفی ایجاد خواهند شد (این مورد برای Docker صدق نمی‌کند).

**تنظیمات پیش‌فرض برای Docker:**

- **نام کاربری:** admin
- **رمز عبور:** admin
- **پورت:** 2053

### مدیریت پایگاه داده:

شما می‌توانید به راحتی پشتیبان‌گیری و بازیابی پایگاه داده را مستقیماً از پنل انجام دهید.

- **مسیر پایگاه داده:**
    - /etc/x-ui/x-ui.db

### مسیر پایه وب

1. **بازنشانی مسیر پایه وب:**
    - ترمینال خود را باز کنید.
    - دستور `x-ui` را اجرا کنید.
    - گزینه `بازنشانی مسیر پایه وب` را انتخاب کنید.

2. **ایجاد یا سفارشی‌سازی مسیر:**
    - مسیر به صورت تصادفی ایجاد می‌شود، یا می‌توانید یک مسیر سفارشی وارد کنید.

3. **مشاهده تنظیمات فعلی:**
    - برای مشاهده تنظیمات فعلی خود، از دستور `x-ui settings` در ترمینال یا `مشاهده تنظیمات فعلی` در `x-ui` استفاده کنید.

### توصیه امنیتی:

- برای امنیت بیشتر، از یک کلمه طولانی و تصادفی در ساختار URL خود استفاده کنید.

**مثال‌ها:**

- `http://ip:port/*webbasepath*/panel`
- `http://domain:port/*webbasepath*/panel`

</details>

## محدودیت IP

<details>
  <summary>برای جزئیات محدودیت IP کلیک کنید</summary>

#### نحوه استفاده

**توجه:** محدودیت IP هنگام استفاده از تونل IP به درستی کار نخواهد کرد.

برای فعال کردن قابلیت محدودیت IP، باید `fail2ban` و فایل‌های مورد نیاز آن را با دنبال کردن این مراحل نصب کنید:

1. دستور `x-ui` را در ترمینال اجرا کنید، سپس `مدیریت محدودیت IP` را انتخاب کنید.
2. گزینه‌های زیر را مشاهده خواهید کرد:

    - **تغییر مدت زمان مسدودیت:** تنظیم مدت زمان مسدودیت‌ها.
    - **رفع مسدودیت همه:** لغو تمام مسدودیت‌های فعلی.
    - **بررسی لاگ‌ها:** بازبینی لاگ‌ها.
    - **وضعیت Fail2ban:** بررسی وضعیت `fail2ban`.
    - **راه‌اندازی مجدد Fail2ban:** راه‌اندازی مجدد سرویس `fail2ban`.
    - **حذف Fail2ban:** حذف Fail2ban به همراه تنظیمات.

3. یک مسیر برای لاگ دسترسی در پنل با تنظیم `Xray Configs/log/Access log` به `./access.log` اضافه کنید، سپس ذخیره کرده و
   xray را مجدداً راه‌اندازی کنید.

</details>

## ربات تلگرام

<details>
  <summary>برای جزئیات ربات تلگرام کلیک کنید</summary>

#### نحوه استفاده

پنل وب از طریق ربات تلگرام از گزارش ترافیک روزانه، ورود به پنل، پشتیبان‌گیری از پایگاه داده، وضعیت سیستم، اطلاعات کاربر
و سایر اعلان‌ها و عملکردها پشتیبانی می‌کند. برای استفاده از ربات، باید پارامترهای مربوط به ربات را در پنل تنظیم کنید، از
جمله:

- توکن تلگرام
- شناسه چت ادمین(ها)
- زمان اعلان (با سینتکس cron)
- اعلان تاریخ انقضا
- اعلان سقف ترافیک
- پشتیبان‌گیری از پایگاه داده
- اعلان بار CPU

**سینتکس مرجع:**

- `30 * * * * *` - اعلان در ثانیه ۳۰ هر دقیقه
- `0 */10 * * * *` - اعلان در ثانیه اول هر ۱۰ دقیقه
- `@hourly` - اعلان ساعتی
- `@daily` - اعلان روزانه (ساعت ۰۰:۰۰ بامداد)
- `@weekly` - اعلان هفتگی
- `@every 8h` - اعلان هر ۸ ساعت

### ویژگی‌های ربات تلگرام

- گزارش دوره‌ای
- اعلان ورود
- اعلان آستانه CPU
- آستانه برای زمان انقضا و ترافیک برای گزارش پیشاپیش
- پشتیبانی از منوی گزارش کلاینت در صورتی که نام کاربری تلگرام کلاینت به تنظیمات کاربر اضافه شده باشد
- پشتیبانی از گزارش ترافیک تلگرام با جستجوی UUID (VMESS/VLESS) یا رمز عبور (TROJAN) - به صورت ناشناس
- ربات مبتنی بر منو
- جستجوی کلاینت با ایمیل (فقط ادمین)
- بررسی تمام ورودی‌ها
- بررسی وضعیت سرور
- بررسی کاربران منقضی شده
- دریافت پشتیبان بر اساس درخواست و در گزارش‌های دوره‌ای
- ربات چند زبانه

### راه‌اندازی ربات تلگرام

- [Botfather](https://t.me/BotFather) را در حساب تلگرام خود شروع کنید:
  ![Botfather](./media/botfather.png)

- یک ربات جدید با استفاده از دستور /newbot ایجاد کنید: از شما ۲ سوال پرسیده می‌شود، یک نام و یک نام کاربری برای ربات
  شما. توجه داشته باشید که نام کاربری باید با کلمه "bot" خاتمه یابد.
  ![Create new bot](./media/newbot.png)

- رباتی را که تازه ایجاد کرده‌اید، شروع کنید. می‌توانید لینک ربات خود را اینجا پیدا کنید.
  ![token](./media/token.png)

- وارد پنل خود شوید و تنظیمات ربات تلگرام را مانند زیر پیکربندی کنید:
  ![Panel Config](./media/panel-bot-config.png)

توکن ربات خود را در فیلد ورودی شماره ۳ وارد کنید.
شناسه کاربری را در فیلد ورودی شماره ۴ وارد کنید. حساب‌های تلگرامی با این شناسه، ادمین ربات خواهند بود. (می‌توانید بیش از
یک شناسه وارد کنید، فقط آنها را با , جدا کنید)

- چگونه شناسه کاربری تلگرام را دریافت کنیم؟ از این [ربات](https://t.me/useridinfobot) استفاده کنید، ربات را شروع کنید و
  شناسه کاربری تلگرام را به شما خواهد داد.
  ![User ID](./media/user-id.png)

</details>

## مسیرهای API

<details>
  <summary>برای جزئیات مسیرهای API کلیک کنید</summary>

#### نحوه استفاده

- [مستندات API](https://www.postman.com/aghayecoder/tx-ui/collection/q1l5l0u/tx-ui)
- `/login` با داده‌های کاربر `POST`: `{username: '', password: ''}` برای ورود
- `/panel/api/inbounds` پایه برای اقدامات زیر:

|  متد   | مسیر                               | عمل                                                      |
|:------:|------------------------------------|----------------------------------------------------------|
| `GET`  | `"/list"`                          | دریافت تمام ورودی‌ها                                     |
| `GET`  | `"/get/:id"`                       | دریافت ورودی با inbound.id                               |
| `GET`  | `"/getClientTraffics/:email"`      | دریافت ترافیک‌های کلاینت با ایمیل                        |
| `GET`  | `"/getClientTrafficsById/:id"`     | دریافت ترافیک کلاینت با شناسه                            |
| `GET`  | `"/createbackup"`                  | ربات تلگرام پشتیبان را برای ادمین‌ها ارسال می‌کند        |
| `POST` | `"/add"`                           | افزودن ورودی                                             |
| `POST` | `"/del/:id"`                       | حذف ورودی                                                |
| `POST` | `"/update/:id"`                    | به‌روزرسانی ورودی                                        |
| `POST` | `"/clientIps/:email"`              | آدرس IP کلاینت                                           |
| `POST` | `"/clearClientIps/:email"`         | پاک کردن آدرس IP کلاینت                                  |
| `POST` | `"/addClient"`                     | افزودن کلاینت به ورودی                                   |
| `POST` | `"/:id/delClient/:clientId"`       | حذف کلاینت با clientId*                                  |
| `POST` | `"/updateClient/:clientId"`        | به‌روزرسانی کلاینت با clientId*                          |
| `POST` | `"/updateClientTraffic/:email"`    | به‌روزرسانی ترافیک کلاینت با ایمیل، مقادیر به بایت هستند |
| `POST` | `"/:id/resetClientTraffic/:email"` | بازنشانی ترافیک کلاینت                                   |
| `POST` | `"/resetAllTraffics"`              | بازنشانی ترافیک تمام ورودی‌ها                            |
| `POST` | `"/resetAllClientTraffics/:id"`    | بازنشانی ترافیک تمام کلاینت‌ها در یک ورودی               |
| `POST` | `"/delDepletedClients/:id"`        | حذف کلاینت‌های منقضی شده ورودی (-1: همه)                 |
| `POST` | `"/onlines"`                       | دریافت کاربران آنلاین (لیست ایمیل‌ها)                    |
| `POST` | `"/depleted"`                      | دریافت کاربران منقضی شده (لیست ایمیل‌ها)                 |
| `POST` | `"/disabled"`                      | دریافت کاربران غیرفعال (لیست ایمیل‌ها)                   |

- فیلد `clientId` باید با موارد زیر پر شود:

- `client.id` برای VMESS و VLESS
- `client.password` برای TROJAN
- `client.email` برای Shadowsocks \.
  `/panel/api/server` پایه برای اقدامات زیر:

|  متد  | مسیر                    | عمل                       |
|:-----:|-------------------------|---------------------------|
| `GET` | `"/status"`             | دریافت وضعیت سرور         |
| `GET` | `"/restartXrayService"` | راه‌اندازی مجدد هسته xray |

[<img src="https://run.pstmn.io/button.svg" alt="Run In Postman" style="width: 128px; height: 32px;">](https://app.getpostman.com/run-collection/5146551-dda3cab3-0e33-485f-96f9-d4262f437ac5?action=collection%2Ffork&source=rip_markdown&collection-url=entityId%3D5146551-dda3cab3-0e33-485f-96f9-d4262f437ac5%26entityType%3Dcollection%26workspaceId%3Dd64f609f-485a-4951-9b8f-876b3f917124)

</details>

## متغیرهای محیطی

<details>
  <summary>برای جزئیات متغیرهای محیطی کلیک کنید</summary>

#### نحوه استفاده

| متغیر          |                  نوع                   | پیش‌فرض     |
|----------------|:--------------------------------------:|:------------|
| XUI_LOG_LEVEL  | "debug" \| "info" \| "warn" \| "error" | "info"      |
| XUI_DEBUG      |               `boolean`                | `false`     |
| XUI_BIN_FOLDER |                `string`                | "bin"       |
| XUI_DB_FOLDER  |                `string`                | "/etc/x-ui" |
| XUI_LOG_FOLDER |                `string`                | "/var/log"  |

مثال:

```sh
XUI_BIN_FOLDER="bin" XUI_DB_FOLDER="/etc/x-ui" go build main.go
```

</details>

## رابط کاربری اشتراک

می‌توانید از این ریپازیتوری برای ایجاد یک رابط کاربری اشتراک برای پنل خود استفاده
کنید [TX-UI Theming Hub](https://github.com/AghayeCoder/TX-ThemeHub)

## تشکر و قدردانی

- از [@Incognito-Coder](https://github.com/incognito-coder) برای مشارکت او در این پروژه
- تشکر ویژه از تمام مشارکت‌کنندگان

## سپاس‌گزاری

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (مجوز: **GPL-3.0**): _قوانین مسیریابی پیشرفته
  v2ray/xray و v2ray/xray-clients با دامنه‌های ایرانی داخلی و تمرکز بر امنیت و مسدود کردن تبلیغات._
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (مجوز: **GPL-3.0**): _این ریپازیتوری شامل
  قوانین مسیریابی V2Ray است که به طور خودکار بر اساس داده‌های دامنه‌ها و آدرس‌های مسدود شده در روسیه به‌روز می‌شود._

## ستاره‌دهندگان در طول زمان

[![Stargazers over time](https://starchart.cc/AghayeCoder/tx-ui.svg?variant=adaptive)](https://starchart.cc/AghayeCoder/tx-ui)
