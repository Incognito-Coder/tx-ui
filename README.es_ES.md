[English](/README.md) | [中文](/README.zh_CN.md) | [Español](/README.es_ES.md) | [Русский](/README.ru_RU.md) | [فارسی](/README.fa_IR.md) | [العربية](/README.ar_EG.md)

<p align="center">
  <picture>
    <img alt="tx-ui" src="./media/tx-ui-dark.png" style="width:512px;height:512px;">
  </picture>
</p>

**Un Panel Web Avanzado • Construido sobre Xray Core** \
**Este proyecto es una bifurcación del panel 3x-ui.**

[![](https://img.shields.io/github/v/release/Incognito-Coder/tx-ui.svg)](https://github.com/Incognito-Coder/tx-ui/releases)
[![](https://img.shields.io/github/actions/workflow/status/Incognito-Coder/tx-ui/release.yml.svg)](#)
[![GO Version](https://img.shields.io/github/go-mod/go-version/Incognito-Coder/tx-ui.svg)](#)
[![Downloads](https://img.shields.io/github/downloads/Incognito-Coder/tx-ui/total.svg)](#)
[![License](https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true)](https://www.gnu.org/licenses/gpl-3.0.en.html)

> **Descargo de responsabilidad:** Este proyecto es solo para aprendizaje personal y comunicación, por favor no lo uses
> con fines ilegales, por favor no lo uses en un entorno de producción

**Si este proyecto te es útil, podrías considerar darle una**:star2:

**USDT(TON):** ```UQD6dwK0jcZuJfH-xMPu4HcnXru9xMKVIzTy_2oZZm-HZzCl```

## Instalar y Actualizar

```
bash <(curl -Ls https://raw.githubusercontent.com/Incognito-Coder/tx-ui/master/install.sh)
```

## Certificado SSL

<details>
  <summary>Haga clic para ver los detalles del certificado SSL</summary>

### ACME

Para gestionar certificados SSL utilizando ACME:

1. Asegúrate de que tu dominio esté correctamente resuelto al servidor.
2. Ejecuta el comando `x-ui` en la terminal y elige `Gestión de Certificados SSL`.
3. Se te presentarán las siguientes opciones:

    - **Get SSL:** Obtener certificados SSL.
    - **Revoke:** Revocar certificados SSL existentes.
    - **Force Renew:** Forzar la renovación de certificados SSL.
    - **Show Existing Domains:** Mostrar todos los certificados de dominio disponibles en el servidor.
    - **Set Certificate Paths for the Panel:** Especificar el certificado para tu dominio que será utilizado por el
      panel.

### Certbot

Para instalar y usar Certbot:

```sh
apt-get install certbot -y
certbot certonly --standalone --agree-tos --register-unsafely-without-email -d yourdomain.com
certbot renew --dry-run
```

### Cloudflare

El script de gestión incluye una aplicación de certificado SSL integrada para Cloudflare. Para usar este script para
solicitar un certificado, necesitas lo siguiente:

- Correo electrónico registrado en Cloudflare
- Clave API Global de Cloudflare
- El nombre de dominio debe estar resuelto al servidor actual a través de Cloudflare

**Cómo obtener la Clave API Global de Cloudflare:**

1. Ejecuta el comando `x-ui` en la terminal y elige `Certificado SSL de Cloudflare`.
2. Visita el enlace: [Tokens de API de Cloudflare](https://dash.cloudflare.com/profile/api-tokens).
3. Haz clic en "Ver Clave API Global" (consulta la captura de pantalla a continuación):
   ![](media/APIKey1.PNG)
4. Es que necesites volver a autenticar tu cuenta. Después de eso, se mostrará la Clave API (consulta la captura
   de pantalla a continuación):
   ![](media/APIKey2.png)

Al utilizarlo, simplemente ingresa tu `nombre de dominio`, `correo electrónico` y `CLAVE API`. El diagrama es el
siguiente:
![](media/DetailEnter.png)

</details>

## Instalación y Actualización Manual

<details>
  <summary>Haz clic para más detalles de la instalación manual</summary>

#### Uso

1. Para descargar la última versión del paquete comprimido directamente en tu servidor, ejecuta el siguiente comando:

```sh
ARCH=$(uname -m)
case "${ARCH}" in
  x86_64 | x64 | amd64) XUI_ARCH="amd64" ;; 
  i*86 | x86) XUI_ARCH="386" ;; 
  armv8* | armv8 | arm64 | aarch64) XUI_ARCH="arm64" ;; 
  armv7* | armv7) XUI_ARCH="armv7" ;; 
  armv6* | armv6) XUI_ARCH="armv6" ;; 
  armv5* | armv5) XUI_ARCH="armv5" ;; 
  *) XUI_ARCH="amd64" ;; 
esac


wget https://github.com/Incognito-Coder/tx-ui/releases/latest/download/x-ui-linux-${XUI_ARCH}.tar.gz
```

2. Una vez que se haya descargado el paquete comprimido, ejecuta los siguientes comandos para instalar o actualizar
   x-ui:

```sh
ARCH=$(uname -m)
case "${ARCH}" in
  x86_64 | x64 | amd64) XUI_ARCH="amd64" ;; 
  i*86 | x86) XUI_ARCH="386" ;; 
  armv8* | armv8 | arm64 | aarch64) XUI_ARCH="arm64" ;; 
  armv7* | armv7) XUI_ARCH="armv7" ;; 
  armv6* | armv6) XUI_ARCH="armv6" ;; 
  armv5* | armv5) XUI_ARCH="armv5" ;; 
  *) XUI_ARCH="amd64" ;; 
esac

cd /root/ 
rm -rf x-ui/ /usr/local/x-ui/ /usr/bin/x-ui
tar zxvf x-ui-linux-${XUI_ARCH}.tar.gz
chmod +x x-ui/x-ui x-ui/bin/xray-linux-* x-ui/x-ui.sh
cp x-ui/x-ui.sh /usr/bin/x-ui
cp -f x-ui/x-ui.service /etc/systemd/system/
mv x-ui/ /usr/local/
systemctl daemon-reload
systemctl enable x-ui
systemctl restart x-ui
```

</details>

## Instalar con Docker

<details>
  <summary>Haz clic para más detalles del Docker</summary>

#### Uso

1. Instala Docker:

   ```sh
   bash <(curl -sSL https://get.docker.com)
   ```

2. Clona el Repositorio del Proyecto:

   ```sh
   git clone https://github.com/Incognito-Coder/tx-ui.git
   cd tx-ui
   ```

3. Inicia el Servicio

   ```sh
   docker compose up -d
   ```

   O tambien

   ```sh
   docker run -itd \
      -e XRAY_VMESS_AEAD_FORCED=false \
      -v $PWD/db/:/etc/x-ui/ \
      -v $PWD/cert/:/root/cert/ \
      --network=host \
      --restart=unless-stopped \
      --name tx-ui \
      ghcr.io/incognitocoder/tx-ui:latest
   ```

actualizar a la última versión

   ```sh
    cd tx-ui
    docker compose down
    docker compose pull tx-ui
    docker compose up -d
   ```

eliminar tx-ui de docker

   ```sh
    docker stop tx-ui
    docker rm tx-ui
    cd --
    rm -r tx-ui
   ```

</details>

## Configuración de Nginx

<details>
  <summary>Haga clic aquí para configurar el proxy inverso</summary>

#### Proxy inverso Nginx

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

#### Nginx sub-path

- EAsegúrese de que la "Ruta Raíz de la URL del Panel" en la configuración del panel `/sub` es la misma.
- El `url` en la configuración del panel debe terminar con `/`.

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

## SO Recomendados

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
- Windows x64 / x86 / ARM64
- macOS (Intel & Apple Silicon M1/M2/M3/M4)
- FreeBSD
- OpenBSD

## Arquitecturas y Dispositivos Compatibles

<details>
  <summary>Haz clic para detalles de arquitecturas y dispositivos compatibles</summary>

Nuestra plataforma ofrece compatibilidad con una amplia gama de arquitecturas y dispositivos, garantizando flexibilidad en diversos entornos informáticos. A continuación se presentan las principales arquitecturas que admitimos:

- **amd64**: Esta arquitectura predominante es la estándar para computadoras personales y servidores, y admite la mayoría de los sistemas operativos modernos.

- **x86 / i386**: Ampliamente adoptada en computadoras de escritorio y portátiles, incluidos sistemas Windows, macOS, Linux, FreeBSD y OpenBSD.

- **armv8 / arm64 / aarch64**: Diseñada para dispositivos móviles, de escritorio y embebidos contemporáneos (Apple Silicon M1/M2/M3/M4, Raspberry Pi 4/5, Orange Pi 3/5, etc.).

- **armv7 / arm / arm32**: Sirve como arquitectura para dispositivos móviles y embebidos como Orange Pi Zero LTS, Orange Pi PC Plus, Raspberry Pi 2/3, entre otros.

- **armv6 / arm / arm32**: Orientada a dispositivos embebidos de bajo consumo (Raspberry Pi 1, Raspberry Pi Zero/Zero W).

- **armv5 / arm / arm32**: Una arquitectura más antigua asociada con sistemas embebidos heredados.

- **mips64 / mips64le / mips32 / mips32le**: Arquitecturas MIPS para enrutadores, hardware de red (plataformas OpenWrt/Linux embebido) y sistemas corporativos.

- **riscv64**: Arquitectura de conjunto de instrucciones abiertas RISC-V de 64 bits para hardware embebido moderno y computadoras de placa única (SiFive, Milk-V, StarFive).

- **ppc64 / ppc64le**: Arquitectura PowerPC de 64 bits para servidores empresariales (sistemas IBM Power).

- **s390x**: Esta arquitectura se utiliza comúnmente en computadoras mainframe de IBM y ofrece alto rendimiento y confiabilidad.

- **loong64**: Arquitectura LoongArch de 64 bits para plataformas Loongson.

</details>

## Idiomas

- Arabic (Árabe)
- English (inglés)
- Persian (persa)
- Traditional Chinese (chino tradicional)
- Simplified Chinese (chino simplificado)
- Japanese (japonés)
- Russian (ruso)
- Vietnamese (vietnamita)
- Spanish (español)
- Indonesian (indonesio)
- Ukrainian (ucraniano)
- Turkish (turco)
- Português (Brazil) (portugués (Brasil))

## Características

- **Documentación REST API Interactiva en el Panel (`/panel/docs`)**: Explora, busca, copia rutas y consulta fragmentos de código interactivos en cURL, Python y Node.js directamente dentro del panel web.
- **Reversión Automática y Manual del Estado de Configuración**: Verificación previa de configuración, detección instantánea de caídas del núcleo y recuperación automática a `config.json.last_working` para evitar interrupciones del servicio.
- **Núcleo Xray Actualizado (v26.10.21)**: Soporte completo para filtrado de IP de origen Finalmask (`allowedSourceIPs`/`blockedSourceIPs`), eliminación automática de filtros en enlaces de suscripción, mejoras de transporte XDRIVE y suites de cifrado TLS ampliadas.
- **Generación y Persistencia Aislada de Credenciales por Protocolo**: Contraseñas dedicadas para Trojan, Shadowsocks 2022 (generación estricta de claves Base64 de 16/32 bytes y autocorrección), MASQUE e Hysteria.
- **Gestión de Dispositivos Conectados**: Rastreo y administración de dispositivos conectados por cliente (IP, aplicación, SO, última conexión) con limpieza de historial.
- **Salud y Mantenimiento de Base de Datos SQLite**: Diagnóstico en panel, truncado de WAL, optimización PRAGMA, desfragmentación VACUUM y limpieza de registros huérfanos.
- Forzar la instalación del panel mediante HTTPS
- Monitoreo del Estado del Sistema (CPU, memoria, swap, disco, sockets TCP/UDP y tráfico en vivo)
- Búsqueda y filtrado dentro de todas las reglas de entrada y clientes
- Tema Oscuro, Ultra Dark y Claro
- Soporta multiusuario y multiprotocolo: VMess, VLESS, Trojan, Shadowsocks, MASQUE, Hysteria, Dokodemo-door, Socks, HTTP, WireGuard
- Soporta Protocolos nativos XTLS: RPRX-Direct, Vision, REALITY
- Estadísticas de tráfico, límite de tráfico, límite de tiempo de vencimiento
- Plantillas de configuración de Xray personalizables
- Soporta acceso HTTPS al panel (dominio propio + certificado SSL)
- Solicitud y renovación automática de certificados SSL con un clic
- Internacionalización completa 100% en 13 idiomas
- Soporta exportar/importar base de datos desde el panel
- Actualizador de aplicaciones integrado

## Configuración Predeterminada del Panel

<details>
  <summary>Haz clic para ver los detalles de la configuración predeterminada</summary>

### Nombre de usuario, Contraseña, Puerto y Ruta Base Web

Si elige no modificar estas configuraciones, se generarán aleatoriamente (esto no se aplica a Docker).

**Configuraciones predeterminadas para Docker:**

- **Nombre de usuario:** admin
- **Contraseña:** admin
- **Puerto:** 2053

### Gestión de la Base de Datos:

Puedes realizar copias de seguridad y restauraciones de la base de datos directamente desde el panel.

- **Ruta de la Base de Datos:**
    - `/etc/x-ui/x-ui.db`

### Ruta Base Web

1. **Restablecer la Ruta Base Web:**
    - Abre tu terminal.
    - Ejecuta el comando `x-ui`.
    - Selecciona la opción `Restablecer la Ruta Base Web`.

2. **Generar o Personalizar la Ruta:**
    - La ruta se generará aleatoriamente, o puedes ingresar una ruta personalizada.

3. **Ver Configuración Actual:**
    - Para ver tu configuración actual, utiliza el comando `x-ui settings` en el terminal o selecciona
      `Ver Configuración Actual` en `x-ui`.

### Recomendación de Seguridad:

- Para mayor seguridad, utiliza una palabra larga y aleatoria en la estructura de tu URL.

**Ejemplos:**

- `http://ip:port/*webbasepath*/panel`
- `http://domain:port/*webbasepath*/panel`

</details>

## Límite de IP

<details>
  <summary>Haz clic para ver los detalles del límite de IP</summary>

#### Uso

**Nota:** El Límite de IP no funcionará correctamente cuando uses Túnel IP.

Para habilitar la funcionalidad de límite de IP, necesitas instalar `fail2ban` y los archivos requeridos siguiendo estos
pasos:

1. Ejecuta el comando `x-ui` en el terminal, luego elige `Gestión de Límite de IP`.
2. Verás las siguientes opciones:

    - **Cambiar la Duración del Bloqueo:** Ajustar la duración de los bloqueos.
    - **Desbloquear a Todos:** Levantar todos los bloqueos actuales.
    - **Revisar los Registros:** Revisar los registros.
    - **Estado de Fail2ban:** Verificar el estado de `fail2ban`.
    - **Reiniciar Fail2ban:** Reiniciar el servicio `fail2ban`.
    - **Desinstalar Fail2ban:** Desinstalar Fail2ban con la configuración.

3. Agrega una ruta para el registro de acceso en el panel configurando `Xray Configs/log/Access log` a `./access.log`,
   luego guarda y reinicia Xray.

</details>

## Bot de Telegram

<details>
  <summary>Haz clic para más detalles del bot de Telegram</summary>

#### Uso

El panel web admite tráfico diario, inicio de sesión en el panel, copia de seguridad de la base de datos, estado del
sistema, información del cliente y otras notificaciones y funciones a través del Bot de Telegram. Para usar el bot,
debes establecer los parámetros relacionados con el bot en el panel, que incluyen:

- Token de Telegram
- ID de chat de administrador(es)
- Hora de Notificación (en sintaxis cron)
- Notificación de Fecha de Caducidad
- Notificación de Capacidad de Tráfico
- Copia de seguridad de la base de datos
- Notificación de Carga de CPU

**Sintaxis de referencia:**

- `30 * * * * *` - Notifica a los 30s de cada punto
- `0 */10 * * * *` - Notifica en el primer segundo de cada 10 minutos
- `@hourly` - Notificación por hora
- `@daily` - Notificación diaria (00:00 de la mañana)
- `@weekly` - Notificación semanal
- `@every 8h` - Notifica cada 8 horas

### Funcionalidades del Bot de Telegram

- Reporte periódico
- Notificación de inicio de sesión
- Notificación de umbral de CPU
- Umbral de Notificación para Fecha de Caducidad y Tráfico para informar con anticipación
- Soporte para menú de reporte de cliente si el nombre de usuario de Telegram del cliente se agrega a las
  configuraciones de usuario
- Soporte para reporte de tráfico de Telegram buscado con UUID (VMESS/VLESS) o Contraseña (TROJAN) - anónimamente
- Bot basado en menú
- Buscar cliente por correo electrónico (solo administrador)
- Ver todas las Entradas
- Ver estado del servidor
- Ver clientes agotados
- Recibir copia de seguridad bajo demanda y en informes periódicos
- Bot multilingüe

### Configuración del Bot de Telegram

- Inicia [Botfather](https://t.me/BotFather) en tu cuenta de Telegram:
  ![Botfather](./media/botfather.png)

- Crea un nuevo bot usando el comando /newbot: Te hará 2 preguntas, Un nombre y un nombre de usuario para tu bot. Ten en
  cuenta que el nombre de usuario debe terminar con la palabra "bot".
  ![Create new bot](./media/newbot.png)

- Inicia el bot que acabas de crear. Puedes encontrar el enlace a tu bot aquí.
  ![token](./media/token.png)

- Ingresa a tu panel y configura los ajustes del bot de Telegram como se muestra a continuación:
  ![Panel Config](./media/panel-bot-config.png)

Ingresa el token de tu bot en el campo de entrada número 3.
Ingresa el ID de chat de usuario en el campo de entrada número 4. Las cuentas de Telegram con esta ID serán los
administradores del bot. (Puedes ingresar más de uno, solo sepáralos con ,)

- ¿Cómo obtener el ID de chat de Telegram? Usa este [bot](https://t.me/useridinfobot), Inicia el bot y te dará el ID de
  chat del usuario de Telegram.
  ![User ID](./media/user-id.png)

</details>

## Rutas de API

<details>
  <summary>Haz clic para más detalles de las rutas de API</summary>

#### Uso

- [Documentación de API](https://www.postman.com/incognito-coder/tx-ui/collection/q1l5l0u/tx-ui)
- `/login` con `POST` datos de usuario: `{username: '', password: ''}` para iniciar sesión
- `/panel/api/inbounds` base para las siguientes acciones:

#### Documentación Completa

- **Documentación Interactiva en el Panel:** Accesible directamente en `/panel/docs` en tu interfaz web de TX-UI.
- **Especificación Completa y Ejemplos de Código:** Consulta [API.md](API.md)
- [Colección de Postman](https://www.postman.com/incognito-coder/tx-ui/collection/q1l5l0u/tx-ui)
- Autenticación: `POST /login` con `{ "username": "admin", "password": "..." }` establece la cookie de sesión.

#### API de Clientes (`/panel/api/clients`)

| Método | Ruta | Acción |
|:------:|------|--------|
| `GET`  | `"/list"` | Obtener todos los clientes con detalles y enlaces de entrada |
| `GET`  | `"/get/:id"` | Obtener cliente por ID |
| `POST` | `"/create"` | Crear nuevo cliente con contraseñas por protocolo (`password`, `ssPassword`, `masquePass`, `auth`) |
| `POST` | `"/bulkCreate"` | Generación masiva de clientes |
| `POST` | `"/update/:id"` | Actualizar configuración de cliente |
| `POST` | `"/del/:id"` | Eliminar cliente |
| `POST` | `"/bulkDel"` | Eliminación masiva de clientes |
| `POST` | `"/:id/toggle"` | Habilitar o deshabilitar cliente |
| `GET`  | `"/:id/links"` | Obtener entradas vinculadas al cliente |
| `POST` | `"/:id/addLink"` | Vincular cliente a entrada |
| `POST` | `"/:id/setLinks"` | Establecer todas las entradas vinculadas |
| `POST` | `"/:id/removeLink/:inboundId"` | Desvincular de entrada específica |
| `GET`  | `"/:id/traffic"` | Estadísticas de tráfico en tiempo real |
| `POST` | `"/:id/resetTraffic"` | Restablecer tráfico del cliente |
| `POST` | `"/resetAllTraffics"` | Restablecer tráfico de todos los clientes |
| `POST` | `"/delDepleted"` | Eliminar clientes vencidos o agotados |
| `GET`  | `"/:id/devices"` | Historial de dispositivos conectados (IP, SO, app) |
| `POST` | `"/:id/clearDevices"` | Limpiar historial de dispositivos |

#### API de Entradas (`/panel/api/inbounds`)

| Método | Ruta | Acción |
|:------:|------|--------|
| `GET`  | `"/list"` | Obtener todas las entradas |
| `GET`  | `"/get/:id"` | Obtener entrada con inbound.id |
| `GET`  | `"/getClientTraffics/:email"` | Obtener tráfico de cliente por email |
| `GET`  | `"/getClientTrafficsById/:id"` | Obtener tráfico de cliente por ID |
| `POST` | `"/add"` | Agregar entrada |
| `POST` | `"/del/:id"` | Eliminar entrada |
| `POST` | `"/update/:id"` | Actualizar entrada |
| `POST` | `"/reorder"` | Reordenar entradas |
| `POST` | `"/clientIps/:email"` | Registro de direcciones IP del cliente |
| `POST` | `"/clearClientIps/:email"` | Limpiar registro de direcciones IP |
| `POST` | `"/updateClientTraffic/:email"` | Actualizar tráfico del cliente manualmente (bytes) |
| `POST` | `"/:id/resetClientTraffic/:email"` | Restablecer tráfico del cliente |
| `POST` | `"/resetAllTraffics"` | Restablecer tráfico de todas las entradas |
| `POST` | `"/resetAllClientTraffics/:id"` | Restablecer tráfico de todos los clientes (-1: todos) |
| `POST` | `"/delDepletedClients/:id"` | Eliminar clientes agotados de la entrada (-1: todos) |
| `POST` | `"/onlines"` | Obtener usuarios en línea (lista de correos) |
| `POST` | `"/depleted"` | Obtener usuarios agotados (lista de correos) |
| `POST` | `"/disabled"` | Obtener usuarios desactivados (lista de correos) |

#### API de Servidor y Sistema (`/panel/api/server`)

| Método | Ruta | Acción |
|:------:|------|--------|
| `GET`  | `"/status"` | Obtener estado de CPU, memoria, disco y núcleo Xray |
| `POST` | `"/restartXrayService"` | Reiniciar servicio del núcleo Xray |
| `POST` | `"/stopXrayService"` | Detener servicio del núcleo Xray |
| `POST` | `"/rollbackXrayConfig"` | Revertir configuración a último estado funcional |
| `GET`  | `"/getRollbackStatus"` | Estado y motivo de reversión de configuración |
| `GET`  | `"/getXrayVersion"` | Versión instalada del núcleo Xray |
| `GET`  | `"/getConfigJson"` | Obtener archivo activo config.json |
| `GET`  | `"/getDb"` | Descargar copia de seguridad de la base de datos |
| `POST` | `"/importDB"` | Restaurar base de datos desde archivo .db |
| `POST` | `"/logs/:count"` | Ver registros del panel |
| `POST` | `"/xraylogs/:count"` | Ver registros del núcleo Xray |
| `GET`  | `"/getNewUUID"` | Generar UUID v4 aleatorio |
| `GET`  | `"/getNewX25519Cert"` | Generar par de claves Reality X25519 |
| `POST` | `"/installPanel"` | Actualizar o instalar paquete del panel |

#### API de Configuración y Mantenimiento (`/panel/setting`)

| Método | Ruta | Acción |
|:------:|------|--------|
| `POST` | `"/all"` | Obtener todas las configuraciones del panel |
| `POST` | `"/update"` | Guardar y actualizar configuraciones |
| `POST` | `"/restartPanel"` | Reiniciar ordenadamente el proceso web del panel |
| `POST` | `"/dbStatus"` | Diagnóstico de base de datos SQLite, tamaño y WAL |
| `POST` | `"/optimizeDatabase"` | Optimizar base de datos (truncar WAL y PRAGMA optimize) |
| `POST` | `"/vacuumDatabase"` | Recuperar espacio en disco mediante VACUUM |
| `POST` | `"/cleanDatabase"` | Buscar y eliminar filas huérfanas o dañadas |

#### API de Configuración Avanzada Xray (`/panel/xray`)

| Método | Ruta | Acción |
|:------:|------|--------|
| `POST` | `"/"` | Obtener configuración avanzada de plantilla Xray JSON |
| `POST` | `"/update"` | Validar y guardar configuración avanzada |
| `POST` | `"/rollback"` | Revertir a la última configuración funcional |
| `GET`  | `"/getRollbackStatus"` | Obtener indicador y motivo de reversión |
| `POST` | `"/testOutbound"` | Probar conectividad y latencia de salidas |
| `GET`  | `"/getOutboundsTraffic"` | Tráfico acumulado por etiqueta de salida |
| `POST` | `"/resetOutboundsTraffic"` | Restablecer contadores de tráfico de salida |

[<img src="https://run.pstmn.io/button.svg" alt="Run In Postman" style="width: 128px; height: 32px;">](https://app.getpostman.com/run-collection/5146551-dda3cab3-0e33-485f-96f9-d4262f437ac5?action=collection%2Ffork&source=rip_markdown&collection-url=entityId%3D5146551-dda3cab3-0e33-485f-96f9-d4262f437ac5%26entityType%3Dcollection%26workspaceId%3Dd64f609f-485a-4951-9b8f-876b3f917124)

</details>

## Variables de Entorno

<details>
  <summary>Haz clic para más detalles de las variables de entorno</summary>

#### Uso

| Variable       |   Tipo    | Predeterminado |
|----------------|:---------:|:---------------|
| XUI_LOG_LEVEL  |  "debug"  | "info"         | "warn" | "error" |
| XUI_DEBUG      | `boolean` | `false`        |
| XUI_BIN_FOLDER | `string`  | "bin"          |
| XUI_DB_FOLDER  | `string`  | "/etc/x-ui"    |
| XUI_LOG_FOLDER | `string`  | "/var/log"     |

Ejemplo:

```sh
XUI_BIN_FOLDER="bin" XUI_DB_FOLDER="/etc/x-ui" go build main.go
```

</details>

## Interfaz de usuario de suscripción

Puedes usar este repositorio para crear una interfaz de usuario de suscripción para tu
panel [TX-UI Theming Hub](https://github.com/Incognito-Coder/TX-ThemeHub)

## Reconocimientos

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (Licencia: **GPL-3.0**): _Reglas de enrutamiento
  mejoradas de v2ray/xray y v2ray/xray-clients con dominios iraníes integrados y un enfoque en seguridad y bloqueo de
  anuncios._
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (License: **GPL-3.0**): _Este repositorio
  contiene reglas de enrutamiento de V2Ray actualizadas automáticamente basadas en datos de dominios y direcciones
  bloqueados en Rusia._

## Estrellas a lo largo del tiempo

[![Stargazers over time](https://starchart.cc/Incognito-Coder/tx-ui.svg?variant=adaptive)](https://starchart.cc/Incognito-Coder/tx-ui)
