# Resolución de problemas

Un breve FAQ. Para la tabla completa de resolución de problemas del lado del host de la copia de VM por SSH (permiso denegado, verificación de clave de host, variables de plantilla faltantes y más), consulta la [guía de copia de VM por SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) en GitHub.

## Algo no está bien conectado

Abre `/spike` en la interfaz web. La comprobación de integración con el host sondea cada montaje y CLI (socket de Docker, libvirt, restic, qemu-img, rclone) e informa de cualquier pieza que falte. Empieza aquí antes de asumir que es un error: un montaje faltante o un host inalcanzable aparece de inmediato.

## No puedo alcanzar la interfaz web

BombVault sirve HTTPS de fábrica en el puerto `3443` (certificado autofirmado), así que abre `https://<your-unraid-ip>:3443`. Acepta el aviso de certificado autofirmado, o pon BombVault detrás de un proxy inverso con tu propio certificado. Si lo ejecutas con `HTTP_ONLY=true`, sirve HTTP en texto plano en el puerto `3000` en su lugar (pensado para uso detrás de un proxy que termina TLS).

## Perdí mi APP_KEY

`APP_KEY` deriva la contraseña del repositorio restic. Sin ella (y sin el kit de recuperación de la clave de cifrado), las copias cifradas no se pueden recuperar. Por eso el Panel insiste en que descargues el kit de recuperación. Consulta [Copia externa y recuperación](offsite-recovery.md). Genera una clave con `openssl rand -hex 32` y guárdala fuera del servidor antes de confiar en ninguna copia.

## La copia de VM no conecta

La copia de VM habla con libvirt por SSH, nunca con un montaje.

- Confirma que SSH está habilitado en el host y que la clave pública de BombVault está autorizada en `/root/.ssh/authorized_keys` (Ajustes, Sistema, Copia de VM por SSH muestra la clave y un botón **Probar conexión**).
- En una red `br0.x` personalizada, establece `LIBVIRT_HOST` a la IP LAN de tu Unraid (el contenedor no puede alcanzar el host mediante `host.docker.internal` ahí). Habilita **Ajustes, Docker, Host access to custom networks**.
- Si cambiaste el puerto SSH de Unraid, establece `LIBVIRT_SSH_PORT` para que coincida.
- El diagnóstico paso a paso completo (prueba de accesibilidad, enrutamiento VLAN, `Permission denied (publickey)`, `Host key verification failed`) está en la [guía de copia de VM por SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## Una instantánea de VM en vivo no se ejecutó

Las instantáneas en vivo necesitan el agente invitado de qemu instalado en la VM y el disco en `/mnt/cache` (o `/mnt/diskX`), no en `/mnt/user`. En una VM apagada, la modalidad en vivo recurre automáticamente a la ordenada. Una copia ordenada apaga la VM, copia los discos y luego la reinicia, de modo que siempre es consistente.

## Una copia falló con "repository is already locked"

Suele tratarse de un bloqueo de restic huérfano dejado atrás cuando el contenedor se actualizó o reinició a mitad de operación. BombVault detecta un bloqueo huérfano de forma demostrable, lo fuerza a limpiar y lo reintenta una vez, automáticamente. Si persiste, usa **Ajustes, Integridad y mantenimiento, Desbloquear** para el dominio afectado con el fin de limpiar un bloqueo obsoleto a mano. Un problema genuino sigue saliendo a la superficie en lugar de quedar oculto.

## Mi copia externa no ocurrió tras una copia

La replicación externa es de mejor esfuerzo por diseño, de modo que un contratiempo externo nunca hace fallar la copia local. Comprueba el calendario externo de ese dominio (Ajustes, Calendarios): un calendario en blanco replica tras cada copia local, mientras que una cadencia envía con menos frecuencia. Usa **Replicar ahora** en la pestaña Externo para una ejecución bajo demanda, y observa el indicador de replicación en el Panel.

## Una restauración se abortó antes de empezar

Antes de detener o eliminar nada, la restauración ejecuta una comprobación de conflictos previa al arranque: verifica que la IP estática del contenedor y los puertos publicados del host estén libres. Si otro contenedor ya ocupa uno, aborta con un mensaje claro y accionable en lugar de dejar una restauración a medias. Libera el puerto o la IP en conflicto y vuelve a intentarlo.

## Una exportación sencilla falló en lugar de escribir un archivo

Si el cifrado age está activado (Ajustes) pero no hay ningún destinatario válido definido, una exportación falla con un error claro en lugar de escribir texto plano. Añade un destinatario válido (una clave pública age o una clave pública SSH), o desactiva el cifrado si quieres que la exportación sea en texto plano. Consulta [Funciones](features.md).

## El contenedor se reinicia constantemente o parece no saludable

BombVault informa de saludable/no saludable desde su propio `/api/health`. Una herramienta de autorreparación (como Autoheal) puede reiniciarlo automáticamente si el motor se atasca alguna vez. Comprueba el registro del contenedor y el informe de `/spike` para conocer la causa subyacente.

## ¿Sigues atascado?

- Lee las páginas completas de [Configuración](configuration.md) y [Copia externa y recuperación](offsite-recovery.md).
- Pregunta en el [hilo de soporte de Unraid](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Abre una [incidencia en GitHub](https://github.com/junkerderprovinz/bombvault/issues).
