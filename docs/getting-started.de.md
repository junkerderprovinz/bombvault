# Erste Schritte

Diese Seite führt dich von einer frischen Unraid-Box bis zu deinem ersten Backup.

## Voraussetzungen

| Voraussetzung | Hinweise |
|---|---|
| **Unraid 6.12+** | Ältere Versionen sind nicht getestet. |
| **Speicherort des restic-Repos** | Ein lokaler Pfad (empfohlen: dein Array oder Cache), SMB, NFS oder ein beliebiges rclone-Backend. |
| **Docker-Socket** | Wird vom Template automatisch eingehängt (`/var/run/docker.sock`). |
| **Unraid-Flash** (`/boot`) | Wird vom Template automatisch komplett eingehängt (`/boot` nach `/host/boot`). Ermöglicht das Flash-Backup und lässt einen wiederhergestellten Container als normale, bearbeitbare Unraid-App wiedererscheinen. |
| **KVM-VMs** (optional) | Das VM-Backup spricht über SSH mit libvirt, kein libvirt-Mount. In den Einstellungen einrichten (siehe [Konfiguration](configuration.md)). |

## Auf Unraid installieren

Der einfachste Weg sind die **Community Applications**.

1. Öffne den **Apps**-Tab in Unraid.
2. Suche nach **BombVault**.
3. Klicke auf **Install**, setze die erforderlichen Variablen (unten) und übernimm.

!!! tip "Manuelle Template-Installation"
    Falls du das Template lieber von Hand hinzufügst:

    1. Gehe zu **Docker, Add Container, Template repositories** und füge hinzu:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Suche in den Templates nach **BombVault**.
    3. Setze die erforderlichen Variablen und klicke auf **Apply**.

## Die eine erforderliche Einstellung

Die einzige Variable, die du setzen musst, ist `APP_KEY`, ein 32-Byte-Hex-Geheimnis (64 Hex-Zeichen), das zum Ableiten des restic-Repository-Passworts dient.

Erzeuge eines auf einer beliebigen Maschine:

```bash
openssl rand -hex 32
```

Füge das Ergebnis in das Feld `APP_KEY` des Templates ein.

!!! danger "Verliere deinen APP_KEY nicht"
    Geht `APP_KEY` verloren, sind deine verschlüsselten Backups unwiederbringlich. Bewahre ihn an einem sicheren Ort getrennt vom Server auf. Sobald BombVault läuft, nutze das Ein-Klick-**Wiederherstellungspaket für den Verschlüsselungsschlüssel** (siehe [Off-site & Wiederherstellung](offsite-recovery.md)), um das vollständige Recovery-Bundle zu speichern.

Das Template hängt außerdem den Docker-Socket, den Flash (`/boot`) und das Wurzelverzeichnis **Host Data** (`/mnt`) für dich ein. Backup-*Quellen* und -*Ziele* liegen beide unter Host Data. Die vollständige Variablenreferenz und die Off-site-Einrichtung findest du unter [Konfiguration](configuration.md).

## Erster Start

1. Öffne die Web-Oberfläche unter `https://<your-unraid-ip>:3443` (selbstsigniertes Zertifikat von Haus aus).
2. Aktiviere in den **Einstellungen** die gewünschten Backup-Bereiche (Container, VMs, Flash, Config, Dateien) und wähle eine Akzentfarbe.
3. Wähle im **Container**-Tab einen Container und klicke auf **Back up**, um deinen ersten Wiederherstellungspunkt zu erstellen. Repository-Pfade sind standardmäßig `/mnt/user/bombvault/{container,vms,flash,config,files}` und werden beim ersten Backup angelegt.
4. Richte die Planung unter **Einstellungen, Zeitpläne** ein. Es gibt ein Ein-Klick-*alle in Zeitplan aufnehmen* für Container und VMs.

!!! tip "Optional: eine Backup-Reihenfolge festlegen"
    Wenn manche Container immer vor anderen gesichert werden sollen (zum Beispiel eine Datenbank vor der App, die sie nutzt), öffne das Panel **backup-order** auf der Container-Seite und ziehe sie in die gewünschte Reihenfolge. Geplante und Mehrfachauswahl-Läufe folgen ihr dann; alles, was du unsortiert lässt, wird wie bisher zuerst nach höchster Überfälligkeit gesichert.

!!! note "Prüfung der Host-Integration"
    Öffne `/spike` in der Web-Oberfläche, nachdem der Container gestartet ist. Es prüft jeden Mount und jedes CLI (Docker-Socket, libvirt, restic, qemu-img, rclone) und meldet fehlende Teile, sodass du bestätigen kannst, dass der Container korrekt verdrahtet ist, bevor du dich darauf verlässt.

## Einfach vs. Erweitert

Standardmäßig zeigt die Oberfläche nur das Wesentliche (sichern, wiederherstellen, planen). Nutze den Schalter **Einfach / Erweitert** in der Seitenleiste, um die Expertensteuerung freizuschalten: Aufbewahrung, Off-site-Kopie, Pre/Post-Hooks, Wiederherstellung auf Dateiebene, Benachrichtigungen, Prometheus-Metriken und die Integritäts-/Wartungswerkzeuge. Es ist eine Einstellung pro Browser und standardmäßig aus, sodass Einsteiger eine aufgeräumte Oberfläche bekommen und Power-User alles.

## Nächste Schritte

- Durchstöbere die vollständigen **[Funktionen](features.md)**.
- Füge eine oder mehrere **[Off-site & Wiederherstellung](offsite-recovery.md)**-Repliken hinzu (jeder Bereich kann gleichzeitig an mehrere Ziele liefern) und speichere dein Recovery-Kit.
- Klonst du ein Setup oder wechselst auf eine neue Box? Nimm deine gesamte Konfiguration mit der Karte **Einstellungen exportieren und importieren** mit. Siehe [Konfiguration](configuration.md#portable-settings-export-and-import).
- Auf ein Problem gestoßen? Siehe **[Fehlerbehebung](troubleshooting.md)**.
