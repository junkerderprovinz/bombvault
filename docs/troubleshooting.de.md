# Fehlerbehebung

Eine kurze FAQ. Die vollständige host-seitige Fehlerbehebungstabelle für VM-über-SSH (permission-denied, Host-Key-Verifizierung, fehlende Template-Variablen und mehr) findest du in der [Anleitung zum VM-Backup über SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) auf GitHub.

## Etwas ist nicht korrekt verdrahtet

Öffne `/spike` in der Web-Oberfläche. Die Prüfung der Host-Integration prüft jeden Mount und jedes CLI (Docker-Socket, libvirt, restic, qemu-img, rclone) und meldet fehlende Teile. Beginne hier, bevor du einen Bug vermutest: ein fehlender Mount oder ein nicht erreichbarer Host zeigt sich sofort.

## Ich erreiche die Web-Oberfläche nicht

BombVault liefert HTTPS von Haus aus auf Port `3443` (selbstsigniertes Zertifikat), öffne also `https://<your-unraid-ip>:3443`. Akzeptiere die Warnung zum selbstsignierten Zertifikat, oder setze BombVault hinter einen Reverse Proxy mit deinem eigenen Zertifikat. Wenn du mit `HTTP_ONLY=true` läufst, liefert es stattdessen schlichtes HTTP auf Port `3000` (gedacht für die Verwendung hinter einem TLS-terminierenden Proxy).

## Ich habe meinen APP_KEY verloren

`APP_KEY` leitet das Passwort des restic-Repositorys ab. Ohne ihn (und ohne das Wiederherstellungspaket für den Verschlüsselungsschlüssel) können verschlüsselte Backups nicht wiederhergestellt werden. Deshalb nervt dich das Dashboard, das Recovery-Kit herunterzuladen. Siehe [Off-site & Wiederherstellung](offsite-recovery.md). Erzeuge einen Schlüssel mit `openssl rand -hex 32` und bewahre ihn off-box auf, bevor du dich auf ein Backup verlässt.

## Das VM-Backup verbindet sich nicht

Das VM-Backup spricht über SSH mit libvirt, nie über einen Mount.

- Bestätige, dass SSH auf dem Host aktiviert ist und BombVaults öffentlicher Schlüssel in `/root/.ssh/authorized_keys` autorisiert ist (Einstellungen, System, VM-Backup über SSH zeigt den Schlüssel und einen Button **Verbindung testen**).
- Setze in einem benutzerdefinierten `br0.x`-Netzwerk `LIBVIRT_HOST` auf deine Unraid-LAN-IP (der Container kann den Host dort nicht über `host.docker.internal` erreichen). Aktiviere **Settings, Docker, Host access to custom networks**.
- Wenn du Unraids SSH-Port geändert hast, setze `LIBVIRT_SSH_PORT` passend.
- Die vollständige Schritt-für-Schritt-Diagnose (Erreichbarkeitstest, VLAN-Routing, `Permission denied (publickey)`, `Host key verification failed`) steht in der [Anleitung zum VM-Backup über SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## Ein Live-VM-Snapshot lief nicht

Live-Snapshots benötigen den in der VM installierten qemu-Gast-Agenten und den Datenträger auf `/mnt/cache` (oder `/mnt/diskX`), nicht `/mnt/user`. Bei einer ausgeschalteten VM fällt Live automatisch auf geordnet zurück. Ein geordnetes Backup fährt die VM herunter, sichert die Datenträger und startet sie dann neu, sodass es immer konsistent ist.

## Ein Backup schlug mit "repository is already locked" fehl

Das ist meist ein verwaister restic-Lock, der zurückblieb, als der Container mitten im Betrieb aktualisiert oder neu gestartet wurde. BombVault erkennt einen nachweislich verwaisten Lock, löst ihn zwangsweise und wiederholt einmal, automatisch. Falls er bestehen bleibt, nutze **Einstellungen, Integrität & Wartung, Entsperren** für den betroffenen Bereich, um einen veralteten Lock von Hand zu lösen. Ein echtes Problem tritt weiterhin zutage, statt verborgen zu werden.

## Meine Off-site-Kopie erfolgte nicht nach einem Backup

Off-site-Replikation ist per Design Best-Effort, sodass ein Off-site-Aussetzer das lokale Backup nie fehlschlagen lässt. Prüfe den Off-site-Zeitplan für diesen Bereich (Einstellungen, Zeitpläne): ein leerer Zeitplan repliziert nach jedem lokalen Backup, während eine Taktung seltener liefert. Nutze **Jetzt replizieren** im Off-site-Tab für einen Lauf auf Abruf, und beobachte die Replikationsanzeige im Dashboard.

## Eine Wiederherstellung brach ab, bevor sie startete

Bevor irgendetwas gestoppt oder entfernt wird, führt die Wiederherstellung eine Vorab-Konfliktprüfung durch: sie verifiziert, dass die statische IP des Containers und die veröffentlichten Host-Ports frei sind. Wenn ein anderer Container einen davon bereits belegt, bricht sie mit einer klaren, handlungsleitenden Meldung ab, statt eine halbfertige Wiederherstellung zu hinterlassen. Gib den konfliktierenden Port oder die IP frei und versuche es erneut.

## Ein Schlichtexport schlug fehl, statt eine Datei zu schreiben

Wenn die age-Verschlüsselung an ist (Einstellungen), aber kein gültiger Empfänger gesetzt ist, schlägt ein Export mit einem klaren Fehler fehl, statt Klartext zu schreiben. Füge einen gültigen Empfänger hinzu (einen age-Public-Key oder einen SSH-Public-Key), oder schalte die Verschlüsselung aus, wenn der Export Klartext sein soll. Siehe [Funktionen](features.md).

## Der Container startet ständig neu oder wirkt ungesund

BombVault meldet gesund/ungesund aus seinem eigenen `/api/health`. Ein Auto-Heal-Werkzeug (wie Autoheal) kann ihn automatisch neu starten, falls sich die Engine je verklemmt. Prüfe das Container-Log und den `/spike`-Bericht auf die zugrunde liegende Ursache.

## Immer noch festgefahren?

- Lies die vollständigen Seiten [Konfiguration](configuration.md) und [Off-site & Wiederherstellung](offsite-recovery.md).
- Frage im [Unraid-Support-Thread](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Öffne ein [GitHub-Issue](https://github.com/junkerderprovinz/bombvault/issues).
