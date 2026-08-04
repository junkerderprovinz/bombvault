# Primi passi

Questa pagina ti accompagna da una macchina Unraid appena installata al tuo primo backup.

## Requisiti

| Requisito | Note |
|---|---|
| **Unraid 6.12+** | Le versioni precedenti non sono testate. |
| **Posizione del repo restic** | Un percorso locale (consigliato: il tuo array o la cache), SMB, NFS o qualsiasi backend rclone. |
| **Socket Docker** | Montato automaticamente dal template (`/var/run/docker.sock`). |
| **Flash Unraid** (`/boot`) | Montato per intero automaticamente dal template (`/boot` in `/host/boot`). Alimenta il backup del flash e permette a un container ripristinato di riapparire come una normale app Unraid, modificabile. |
| **VM KVM** (opzionale) | Il backup delle VM comunica con libvirt via SSH, nessun mount libvirt. Configuralo in Impostazioni (vedi [Configurazione](configuration.md)). |

## Installazione su Unraid

Il percorso più semplice è **Community Applications**.

1. Apri la scheda **Apps** in Unraid.
2. Cerca **BombVault**.
3. Clicca **Install**, imposta le variabili richieste (sotto) e applica.

!!! tip "Installazione manuale del template"
    Se preferisci aggiungere il template a mano:

    1. Vai su **Docker, Add Container, Template repositories** e aggiungi:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Cerca **BombVault** in Templates.
    3. Imposta le variabili richieste e clicca **Apply**.

## L'unica impostazione richiesta

L'unica variabile che devi impostare è `APP_KEY`, un segreto esadecimale di 32 byte (64 caratteri esadecimali) usato per derivare la password del repository restic.

Generane uno su qualsiasi macchina:

```bash
openssl rand -hex 32
```

Incolla il risultato nel campo `APP_KEY` del template.

!!! danger "Non perdere la tua APP_KEY"
    Perdere `APP_KEY` rende i tuoi backup cifrati irrecuperabili. Conservala in un luogo sicuro e separato dal server. Una volta che BombVault è in esecuzione, usa il suo **kit di ripristino della chiave di crittografia** con un clic (vedi [Off-site e ripristino](offsite-recovery.md)) per salvare l'intero pacchetto di ripristino.

Il template monta anche il socket Docker, il flash (`/boot`) e la radice **Host Data** (`/mnt`) per te. Le *origini* e le *destinazioni* dei backup risiedono entrambe sotto Host Data. Per il riferimento completo delle variabili e la configurazione off-site, vedi [Configurazione](configuration.md).

## Prima esecuzione

1. Apri l'interfaccia web all'indirizzo `https://<your-unraid-ip>:3443` (certificato autofirmato pronto all'uso).
2. In **Impostazioni**, abilita i domini di backup che vuoi (Container, VM, Flash, Config, File) e scegli un colore di accento.
3. Nella scheda **Container**, scegli un container e clicca **Backup** per creare il tuo primo punto di ripristino. I percorsi dei repository hanno come predefinito `/mnt/user/bombvault/{container,vms,flash,config,files}` e vengono creati al primo backup.
4. Imposta la pianificazione da **Impostazioni, Calendari**. C'è un *includi tutto nel calendario* con un clic per container e VM.

!!! tip "Facoltativo: scegli un ordine di backup"
    Se alcuni container dovrebbero sempre essere sottoposti a backup prima di altri (per esempio un database prima dell'app che lo usa), apri il pannello **ordine di backup** nella pagina Container e trascinali nella sequenza che vuoi. Le esecuzioni pianificate e a selezione multipla la seguono quindi; tutto ciò che lasci non ordinato viene sottoposto a backup dal più in ritardo per primo, come prima.

!!! note "Verifica dell'integrazione host"
    Apri `/spike` nell'interfaccia web dopo l'avvio del container. Sonda ogni mount e CLI (socket Docker, libvirt, restic, qemu-img, rclone) e segnala eventuali pezzi mancanti, così puoi confermare che il container sia collegato correttamente prima di affidartici.

## Semplice vs Avanzata

Per impostazione predefinita l'interfaccia mostra solo l'essenziale (backup, ripristino, pianificazione). Usa l'interruttore **Semplice / Avanzata** nella barra laterale per rivelare i controlli esperti: conservazione, copia off-site, hook pre/post, ripristino a livello di file, notifiche, metriche Prometheus e gli strumenti di integrità/manutenzione. È una preferenza per browser e disattivata di default, così i nuovi arrivati ottengono un'interfaccia pulita e gli utenti esperti ottengono tutto.

## Prossimi passi

- Sfoglia l'insieme completo delle **[Funzionalità](features.md)**.
- Aggiungi una o più repliche **[Off-site e ripristino](offsite-recovery.md)** (ogni dominio può spedire a più destinazioni contemporaneamente) e salva il tuo kit di ripristino.
- Stai clonando una configurazione o passando a una nuova macchina? Porta con te l'intera configurazione con la scheda **Esporta e importa impostazioni**. Vedi [Configurazione](configuration.md#portable-settings-export-and-import).
- Hai incontrato un intoppo? Vedi **[Risoluzione dei problemi](troubleshooting.md)**.
