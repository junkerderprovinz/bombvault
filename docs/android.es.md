# App de Android

La app de Android lleva a tu teléfono todos los servidores BombVault de tu grupo. Arranca con una lista de tus servidores y, encima, el registro de actividad de todos ellos, las mismas líneas que muestra el resumen, y abre la vista de teléfono del servidor que toques. La app no copia nada por sí misma.

![La pantalla de inicio de la app: el registro de actividad de todos los servidores arriba y debajo la lista de servidores.](assets/screenshots/android/start-dark.png#only-dark)
![La pantalla de inicio de la app: el registro de actividad de todos los servidores arriba y debajo la lista de servidores.](assets/screenshots/android/start-light.png#only-light)

## Cómo conseguir la app {#install}

- **Ajustes, Apps:** la tarjeta de la app de Android ofrece el APK de la versión que ejecuta tu servidor, con un código QR para escanear desde el teléfono, y enlaza la ficha de F-Droid.
- **APK:** cada versión tiene `bombvault-android.apk` en su página de lanzamiento, y [este enlace](https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk) descarga siempre la última compilación. Necesita Android 10 o posterior, y Android te pregunta una vez si la aplicación con la que abres el archivo puede instalar aplicaciones.
- **Google Play:** la app está en una prueba cerrada hasta que pueda hacerse pública. Google Play solo muestra una app de una cuenta de desarrollador nueva cuando al menos 12 probadores la han mantenido instalada durante 14 días. Para ayudar, únete al [grupo de probadores](https://groups.google.com/g/arrowloop-testers), abre la [página de prueba](https://play.google.com/apps/testing/bombvault.halleluja.design), toca **Become a tester** e instala BombVault desde Google Play.
- **F-Droid:** [BombVault en F-Droid](https://f-droid.org/packages/bombvault.halleluja.design/). F-Droid compila la app a partir del código fuente y la firma con su propia clave, así que una versión nueva le llega unos días más tarde. Para cambiar entre la app de F-Droid y el APK, desinstala antes la otra.

Empareja la app con servidores de la versión 9.7.0 o posterior. Un servidor de una versión anterior puede estar en el mismo grupo, pero muestra el teléfono como una instancia normal, y la app solo lee la actividad de ese servidor después de iniciar sesión.

## Emparejar con un código QR {#pairing}

1. En cualquier servidor de tu grupo, abre **Ajustes, Emparejamiento** y elige **Mostrar frase**. Aparecen las doce palabras con un código QR al lado.
2. En la app, toca **Escanear código QR** y apunta el teléfono al código. También puedes pegar o escribir las palabras.
3. La app muestra la lista de servidores de ese grupo antes de guardar nada. **Añadir todos** añade todos de una vez.

Después el teléfono se une al grupo como una instancia más. Lee lo que se ejecuta en cada servidor a través del grupo sin iniciar sesión, en casa directamente y fuera de casa mediante el repetidor. Cómo funciona el grupo en sí se explica en [Emparejar instancias](offsite-recovery.md#pairing).

La app se une al grupo a través del repetidor, también en casa. Si tus instancias usan **Sin repetidor**, elige uno en **Ajustes, Emparejamiento** antes de escanear el código.

## Fuera de casa {#away}

La app prueba primero un servidor en su propia dirección. Cuando no responde, como ocurre con la red móvil, abre en su lugar la interfaz del servidor a través del repetidor del grupo. Un servidor del grupo cuya dirección la app aún no conoce se abre de la misma forma. Si no responden ni la dirección ni el repetidor, la pantalla de inicio lo indica en la tarjeta del servidor.

Por el repetidor la página llega a lo mismo que alcanza un teléfono emparejado en casa, como se describe más abajo. Iniciar sesión con la contraseña y guardar archivos solo funcionan en la propia dirección del servidor. Las barras de progreso se actualizan cada dos segundos en lugar de de forma continua. Los archivos de la interfaz se quedan en el teléfono tras la primera visita, así que las siguientes cargan más rápido. Un servidor anterior a la 10.0.0 solo se abre en su propia dirección.

La app solo acepta una respuesta si el servidor la firmó con la clave que la app fijó cuando conoció a ese servidor. Si más tarde el servidor responde con otra clave, la pantalla de inicio muestra la huella de la clave nueva y la página solo se abre después de que confíes en ella.

## Cambiar de servidor {#switch}

Con un servidor abierto, la parte superior de su resumen muestra el nombre del servidor, con una flecha de retroceso delante. La flecha lleva al resumen de la app, la pantalla de inicio, que lista todos los servidores que la app conoce con su estado: **Conectado**, **Sin conexión**, **Inicio de sesión necesario** o **Comprobar certificado**. Un servidor del grupo que no responde figura como **Inactiva**, en gris, con la última vez que el grupo lo vio, y uno del que la app aún no ha sabido nada dice **Comprobando**. Cada vez que se conecta, la app lee la lista del grupo de una de sus instancias, así que también aparece una instancia que nunca conoció por el repetidor, por ejemplo una que estaba apagada cuando el teléfono se unió. Toca otro servidor ahí para abrirlo, directamente en casa y por el repetidor en cualquier otro sitio.

## Con sesión iniciada en un teléfono emparejado {#sign-in}

Un teléfono emparejado con tu grupo abre cada uno de sus servidores con la sesión ya iniciada. Antes de cargar una página, pide a ese servidor una sesión a través del grupo, y un servidor solo la entrega a un miembro que sea un teléfono.

Un teléfono emparejado no recibe todo lo que da una contraseña. Su sesión llega a una lista fija de lo que muestra la app: el resumen, las ejecuciones y el registro de actividad, las copias para consultar, iniciar y restaurar, las anomalías para leer y la página Instancias para consultar, sin los repositorios recibidos ni las fuentes de recogida de sus miembros. Los destinos y los comandos se devuelven sin sus direcciones ni secretos. Una restauración que reemplaza el original copia antes el estado actual. Una restauración en una carpeta va a una carpeta nueva dentro de la carpeta de restauración fijada en **Ajustes, Restauración**, y la ventana no ofrece ninguna otra. Las claves, el kit de recuperación, los depósitos de claves, los ajustes y el borrado de copias requieren iniciar sesión en el propio servidor, y todo lo demás responde que necesita ese inicio de sesión. Por el repetidor se aplica la misma lista, también en un servidor sin contraseña.

El interruptor para esto es **Los teléfonos pueden emparejarse con las doce palabras del grupo** en la tarjeta **App de Android** de **Ajustes, Seguridad**. Un restablecimiento de contraseña lo desactiva y cierra la sesión de todos los teléfonos, hasta que lo vuelvas a activar ahí.

## Certificados {#certificates}

Un certificado autofirmado se acepta una vez por su huella SHA-256. Cuando el servidor muestra más tarde otro distinto, la app te avisa y solo lo abre después de que confíes en el certificado nuevo.

## El teléfono en la página Instancias {#instances}

El teléfono tiene su propia tarjeta en la página Instancias de cada servidor del grupo, marcada como app de Android y con el nombre que le diste al teléfono. No tiene marcador porque no copia nada. **Eliminar** la quita de la página y termina su sesión al instante, igual que desactivar **Consultar esta instancia** en **Supervisión** dentro de sus detalles. El teléfono sigue conociendo las palabras, así que vuelve a aparecer cuando se abre su app, pero desactivado, y no recibe sesión hasta que alguien lo active. Si pierdes un teléfono, elimina su tarjeta en cada servidor, o desactiva **Los teléfonos pueden emparejarse con las doce palabras del grupo** en **Ajustes, Seguridad**, o deja que cada instancia use **Salir del grupo** y empareja de nuevo con palabras nuevas.

Un teléfono emparejado antes de la 10.0.0 empieza a firmar sus llamadas en su primera visita tras la actualización, y cada servidor se queda con la primera clave que recibe del teléfono. Si otra persona que conoce las palabras del grupo llega antes mientras el teléfono está sin conexión, ese servidor rechaza el teléfono desde ese momento y la app ya no puede abrirlo por el repetidor. Para arreglarlo, inicia sesión en ese servidor en un navegador, elige **Olvidar la clave** en la tarjeta del teléfono de la página Instancias y luego **Confirmar olvido**, y abre la app otra vez. El servidor toma la clave propia del teléfono en su siguiente llamada. Si el teléfono vuelve a ser rechazado, otra persona tiene las palabras: deja que cada instancia use **Salir del grupo** y empareja de nuevo con palabras nuevas, como con un teléfono perdido.

## Ajustes {#settings}

El engranaje junto al signo más abre los ajustes de la app:

- el idioma y el nombre que muestra el teléfono en la página Instancias (vacío significa el modelo del teléfono),
- el aspecto, que sigue al primer servidor de la lista hasta que pongas el tuyo, y las animaciones, que tienen su propio ajuste,
- un informe para copiar cuando comuniques un problema. No contiene ninguna dirección, nombre ni frase,
- la tarjeta Acerca de con la política de privacidad,
- **Quitar todos los servidores**, que quita todos los servidores de la app y sale del grupo. Cada servidor al que el teléfono aún llega lo elimina de su página Instancias.

## Descargas y subidas {#files}

Las exportaciones, los kits de recuperación, los ZIP de Flash y los volcados de la base de datos van a la carpeta Descargas del teléfono, como desde un navegador. Importar ajustes abre el selector de archivos del teléfono. Por el repetidor no se guarda ningún archivo. Para eso abre el servidor en su propia dirección.

## En una pantalla táctil {#touch}

Bajo un dedo nada se resalta al pasar por encima, así que un control se atenúa mientras lo mantienes pulsado, y un botón con el logotipo de una marca se ilumina con el color de la marca hasta que levantas el dedo. Una pulsación larga en un botón cuenta como un toque lento y no abre ningún menú de enlaces. Un toque en un botón en gris muestra por qué está en gris, hasta que toques en otro sitio.

## En un navegador {#browser}

Chrome y Edge pueden instalar la interfaz web de BombVault como una app en su propia ventana, en el teléfono igual que en el ordenador. No se guarda nada en caché, así que una actualización se ve al instante.

## Privacidad {#privacy}

La app no tiene cuentas, ni publicidad, ni analíticas, y no ejecuta nada en segundo plano. Su [política de privacidad](https://github.com/junkerderprovinz/bombvault/blob/main/android/PRIVACY.md) enumera lo que guarda y lo que envía y adónde.
