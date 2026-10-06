# App de Android

La app de Android lleva todos los servidores BombVault de tu grupo al móvil. Arranca con una lista de tus servidores y, encima, el registro de actividad de todos ellos, las mismas líneas que muestra el Panel, y abre la vista móvil del servidor que toques. La app no hace ninguna copia por sí misma.

## Conseguir la app {#install}

- **Ajustes, Apps:** la tarjeta de la app de Android ofrece el APK de la versión que ejecuta tu servidor, con un código QR para escanear desde el móvil.
- **APK:** cada versión publica `bombvault-android.apk` en su página de release, y [este enlace](https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk) descarga siempre la última compilación. Necesita Android 10 o posterior, y Android pregunta una vez si la app con la que abres el archivo puede instalar apps.
- **Google Play:** la app está en una prueba cerrada hasta que pueda publicarse. Google Play solo publica una app de una cuenta de desarrollador nueva después de que al menos 12 testers la hayan mantenido instalada durante 14 días. Para ayudar, únete al [grupo de testers](https://groups.google.com/g/arrowloop-testers), abre la [página de prueba](https://play.google.com/apps/testing/bombvault.halleluja.design), toca **Convertirse en tester** e instala BombVault desde Google Play.
- **F-Droid:** la ficha llegará más adelante.

Empareja la app con servidores en la versión 9.7.0 o posterior. Un servidor con una versión anterior puede estar en el mismo grupo, pero muestra el móvil como una instancia normal, y la app solo lee la actividad de ese servidor después de iniciar sesión.

## Emparejar con un código QR {#pairing}

1. En cualquier servidor de tu grupo, abre **Ajustes, Emparejamiento** y elige **Mostrar frase**. Las doce palabras aparecen con un código QR al lado.
2. En la app, toca **Escanear código QR** y apunta el móvil al código. También puedes pegar o escribir las palabras.
3. La app lista los servidores de ese grupo antes de guardar nada. **Añadir las N** los añade todos.

A partir de ahí el móvil se une al grupo como una instancia más. Lee lo que se ejecuta en cada servidor a través del grupo sin iniciar sesión, directamente en casa y a través del relay fuera de casa. Cómo funciona el propio grupo se describe en [Emparejamiento de instancias](offsite-recovery.md#pairing).

!!! note "La interfaz sigue necesitando un camino hasta el servidor"
    La lista de servidores y el registro de actividad llegan a través del grupo. La interfaz de un servidor se abre directamente, así que el móvil tiene que poder llegar a la dirección del servidor, en casa o por una VPN.

## Sesión iniciada en un móvil emparejado {#sign-in}

Un móvil emparejado con tu grupo abre cada uno de sus servidores con la sesión ya iniciada. Antes de cargar una página, pide una sesión a ese servidor a través del grupo, y un servidor solo la concede a un miembro que sea un móvil. Un servidor añadido por su dirección pide la contraseña, como en un navegador. Cualquiera que tenga las doce palabras ya puede abrir todas las copias del grupo, así que el emparejamiento no concede nada nuevo.

## Servidores fuera de un grupo {#other-servers}

- **Añadir servidor** acepta la dirección con la que abres BombVault en un navegador, por ejemplo `192.168.1.10:3443`. Sin `http://` ni `https://` delante, la app usa https.
- Los servidores que se anuncian en la red local aparecen en **En esta red** y se abren con un toque. Lo hacen mientras **Encontrar en la red** está activado en Ajustes, Integraciones. Un servidor en otra red o detrás de una VPN no aparece ahí.
- Un certificado autofirmado se acepta una vez por su huella SHA-256. Si más adelante el servidor presenta otro distinto, la app te avisa y solo lo abre después de que confíes en el certificado nuevo.

## El móvil en la página de Instancias {#instances}

El móvil recibe su propia tarjeta en la página de Instancias de cada servidor del grupo, marcada como app de Android y con el nombre que le diste al móvil. No tiene cuadro de mando porque no hace copias, y **Eliminar** la quita de la página.

## Ajustes {#settings}

El engranaje junto al botón + abre los ajustes de la app:

- el idioma y el nombre que muestra el móvil en la página de Instancias (vacío significa el modelo del móvil),
- el aspecto, que sigue al primer servidor de la lista hasta que defines el tuyo, y las animaciones, que tienen su propio ajuste,
- un informe para copiar cuando comuniques un problema; no contiene ninguna dirección, nombre ni frase,
- la tarjeta **Acerca de** con la política de privacidad,
- **Quitar todos los servidores**, que quita todos los servidores de la app y abandona el grupo. En los propios servidores no cambia nada.

## Descargas y subidas {#files}

Las exportaciones, los kits de recuperación, los ZIP del flash y los volcados de bases de datos van a la carpeta Descargas del móvil, como desde un navegador. Una importación de ajustes abre el selector de archivos del móvil.

## En una pantalla táctil {#touch}

Bajo un dedo no hay nada que pasar por encima, así que un control se atenúa mientras se mantiene pulsado, y un botón con el logotipo de una marca se ilumina en el color de esa marca hasta que se levanta el dedo. Una pulsación larga en un botón cuenta como un toque lento y no abre ningún menú de enlace.

## O en un navegador {#browser}

Chrome y Edge pueden instalar la interfaz web de BombVault como una app en su propia ventana, tanto en un móvil como en un ordenador. No se guarda nada en caché, así que una actualización se ve al instante.

## Privacidad {#privacy}

La app no tiene cuentas, ni publicidad, ni analíticas, y no ejecuta nada en segundo plano. Su [política de privacidad](https://github.com/junkerderprovinz/bombvault/blob/main/android/PRIVACY.md) detalla qué guarda y qué envía y adónde.
