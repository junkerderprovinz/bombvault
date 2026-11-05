# Kopie maszyn wirtualnych

BombVault kopiuje maszyny wirtualne KVM/libvirt bez montowania jakiejkolwiek ścieżki libvirt. Uruchamia `virsh` na hoście przez SSH (`qemu+ssh://`), odczytuje i zapisuje dyski maszyn przez montowanie Host Data (`/mnt` na hoście to `/host/user` w kontenerze) i kopiuje UEFI NVRAM przez SSH. Ponieważ nic, co należy do libvirt, nie jest montowane, BombVault nigdy nie wejdzie w drogę VM Managerowi w Unraid.

Ten przewodnik obejmuje konfigurację, metody kopii, maszyny z migawkami Unraid i TrueNAS SCALE. Kopie kontenerów nie potrzebują niczego z tego.

Kopie zbiorów danych ZFS używają tego samego łącza SSH, z tym samym kluczem, hostem i użytkownikiem. Gdy kopie maszyn wirtualnych się łączą, łączą się też elementy ZFS. Zobacz [Zbiory danych ZFS](zfs-datasets.md).

## Co konfigurujesz {#what-you-configure}

| Ustawienie | Gdzie | Domyślnie | Znaczenie |
|---|---|---|---|
| **Host SSH: Address** (`LIBVIRT_HOST`) | zmienna szablonu | `host.docker.internal` | Host Unraid, osiągany przez SSH |
| **Host SSH: Port** (`LIBVIRT_SSH_PORT`) | zmienna szablonu | `22` | Port SSH Unraid |
| **Host SSH: User** (`LIBVIRT_SSH_USER`) | zmienna szablonu | `root` | Użytkownik SSH na hoście |
| Klucz publiczny | **Ustawienia, Integracje, SSH hosta** | tworzony przy pierwszym starcie | Autoryzuj go na hoście |

BombVault tworzy swoją parę kluczy SSH przy pierwszym starcie w `/config/ssh/id_ed25519`, w appdata. Klucz hosta jest przypinany w `/config/ssh/known_hosts` przy pierwszym połączeniu.

Szablon wypełnia **Host SSH: Address** zastępczym adresem `192.168.x.x`. BombVault traktuje go jako nieustawiony i próbuje `host.docker.internal`, co działa w domyślnej sieci `bridge`. W sieci niestandardowej zastąp go adresem IP hosta w LAN (krok 3 B).

## Krok 1: Włącz SSH na hoście Unraid {#step-1}

1. W Unraid otwórz **Settings, Management Access** i ustaw **Use SSH** na **Yes**.
2. Jeśli zmieniłeś port SSH, zapamiętaj go. **Host SSH: Port** musi się zgadzać.

## Krok 2: Autoryzuj klucz publiczny BombVault {#step-2}

`/root` w Unraid leży w pamięci i jest czyszczone przy każdym uruchomieniu, więc klucz musi leżeć także na dysku flash. Uruchom to w terminalu Unraid:

```sh
K=$(docker exec BombVault cat /config/ssh/id_ed25519.pub)
mkdir -p /root/.ssh /boot/config/ssh && chmod 700 /root/.ssh
grep -qxF "$K" /root/.ssh/authorized_keys 2>/dev/null  || echo "$K" >> /root/.ssh/authorized_keys
grep -qxF "$K" /boot/config/ssh/root.pubkeys 2>/dev/null || echo "$K" >> /boot/config/ssh/root.pubkeys
chmod 600 /root/.ssh/authorized_keys
```

Unraid 6.9 i nowszy kopiują `/boot/config/ssh/root.pubkeys` do `authorized_keys` roota przy każdym uruchomieniu, więc to przetrwa restarty. Działa dla każdego logowania `root`, bez względu na nazwę hosta czy port, których używa BombVault.

## Krok 3: Sieć {#step-3}

BombVault musi otworzyć połączenie TCP z kontenera do portu SSH hosta. Wybierz przypadek, który pasuje do Twojej konfiguracji.

### A. BombVault w domyślnej sieci bridge {#bridge}

- Zostaw **Host SSH: Address** na `host.docker.internal` albo na zastępczym adresie z szablonu. Szablon dodaje `--add-host=host.docker.internal:host-gateway`, co wskazuje bramę docker0, czyli host.
- Ustaw **Host SSH: Port** na swój port SSH.

### B. BombVault w sieci niestandardowej {#custom-network}

Kontener w `br0.x`, macvlan albo ipvlan nie dotrze do hosta przez `host.docker.internal`, bo 172.17.0.1 to docker0 i nie jest osiągalny z `br0.x`. Zamiast tego:

1. W Unraid ustaw **Settings, Docker, Host access to custom networks** na **Enabled**.
2. Ustaw **Host SSH: Address** na adres IP hosta Unraid w LAN, ten, pod którym otwierasz interfejs webowy.
3. Jeśli sieć kontenera i host są w różnych VLAN, zezwól na trasę na routerze lub zaporze: z VLAN kontenera do IP hosta na porcie SSH, przez TCP.
4. Jeśli adresu IP hosta w LAN nadal nie da się osiągnąć, użyj adresu shim hosta w podsieci kontenera. Znajdziesz go na hoście poleceniem `ip -4 addr show | grep -B2 <container-subnet>`.

### Sprawdź, czy host odpowiada {#verify-reachability}

Uruchom to w terminalu Unraid, ze swoim IP i portem:

```sh
docker exec BombVault timeout 6 bash -c 'echo > /dev/tcp/192.168.1.10/22' && echo OPEN || echo UNREACHABLE
```

`OPEN` znaczy, że ścieżka działa. `UNREACHABLE` znaczy, że sieć z kroku 3 trzeba naprawić przed czymkolwiek innym.

## Krok 4: Ustaw zmienne i przetestuj {#step-4}

1. W Unraid otwórz **Docker**, kliknij BombVault, wybierz **Edit**, ustaw **Host SSH: Address** (i **Host SSH: Port**, jeśli to nie 22) i naciśnij **Apply**. Jeśli zmiennych brakuje, zaimportuj szablon ponownie, bo Unraid zachowuje zapisaną konfigurację istniejącego kontenera.
2. W BombVault otwórz **Ustawienia, Integracje, SSH hosta** i naciśnij **Testuj połączenie**. Powinno zrobić się zielone.

To samo wywołanie z terminala wymienia Twoje maszyny wirtualne:

```sh
docker exec BombVault virsh -c "qemu+ssh://root@192.168.1.10:22/system?keyfile=/config/ssh/id_ed25519&known_hosts=/config/ssh/known_hosts&known_hosts_verify=auto" list --all
```

## Krok 5: Metoda dla każdej maszyny {#per-vm-method}

Każda maszyna ma **Metoda** w karcie **VM** na swojej karcie Ustawienia (otwórz maszynę w **Kopie zapasowe**):

- **Graceful (zamknięcie)** jest domyślna. Zamyka maszynę, kopiuje dyski i uruchamia ją z powrotem, więc kopia jest zawsze spójna. Czeka do dziesięciu minut, aż gość się zamknie, i nigdy nie wyłącza go siłą. Gość, który nadal działa po tym czasie, zostaje w spokoju, a ta kopia kończy się błędem z powodem. Jeśli gość wyłączy się w ciągu następnej godziny, BombVault uruchomi go ponownie. Jeśli to uruchomienie się nie uda, mówi o tym nieudany przebieg.
- **Migawka na żywo** kopiuje działającą maszynę przez zewnętrzną migawkę i `blockcommit`. Wymaga dysku na `/mnt/cache` albo `/mnt/diskX`, a nie `/mnt/user`. Z agentem gościa qemu w maszynie migawka jest wykonywana z wyciszeniem, a bez niego jest spójna na poziomie awarii. Na wyłączonej maszynie Migawka na żywo sama cofa się do Graceful. Gdy migawki nie da się wykonać, ta kopia kończy się błędem, a maszyna działa dalej, bo maszyna, którą wybrałeś do kopii na żywo, nigdy nie jest dla niej zamykana.

## Czego kopia nie obejmuje {#left-out}

Kopia może się udać, a mimo to nie zawierać całej maszyny. Przebieg ma wtedy uwagę w tonie ostrzeżenia, która wymienia, czego brakuje:

- **Dyski, których BombVault nie czyta.** Dysk tylko do odczytu, `lun` oraz dysk `network` lub `volume` są pomijane, a uwaga wymienia każdy z nich z jego urządzeniem docelowym i rodzajem. Definicja w kopii nadal je wymienia. Napęd cdrom i stacja dyskietek nie są wymieniane. Zwykłe urządzenie blokowe, które nie jest zvol ZFS, zamiast tego kończy kopię błędem.
- **Stan emulowanego TPM.** Nie ma ścieżki w XML domeny. VM Manager w Unraid ustawia taki TPM dla maszyny z BIOS-em **OVMF TPM**, więc uwaga pojawia się także na Unraid. Gość, który trzyma sekret w swoim TPM, jak potrafi BitLocker, może po przywróceniu, które traci ten stan, poprosić o swój klucz odzyskiwania, więc miej ten klucz pod ręką.

## Cienkie dyski {#thin-disks}

Przywracanie zachowuje cienki dysk jako cienki, czy zastępuje dysk, czy zapisuje nowy: restic zapisuje ciągi zer jako dziury w nowym pliku, a BombVault zwalnia bloki zer zastąpionego pliku przez `fallocate --dig-holes`, gdy plik jest już zapisany. Gdy restic zastępuje plik, ten plik zajmuje pełny rozmiar, jeśli pula ma na to miejsce. System plików bez wybijania dziur zostawia dysk w pełnym rozmiarze, a log o tym mówi. Sprawdzenie miejsca nadal liczy nowy dysk w pełnym rozmiarze.

## Przywracanie, które się nie udaje {#restore-fails}

Przywracanie na miejscu wyłącza maszynę, zapisuje dyski i zastępuje definicję oraz NVRAM dopiero wtedy, gdy dyski są zapisane. Przywracanie, które się nie uda, zostawia maszynę zdefiniowaną razem z jej NVRAM. Jeśli jej dyski się nie zmieniły, a działała, uruchamia się ponownie. Jeśli mogły się zmienić, zostaje wyłączona, bo uruchomienie z zapisanego w połowie dysku pisałoby po nim, a przebieg to mówi. Ponowne przywrócenie tej samej kopii kończy robotę.

Migawki zapisane w libvirt (VM Manager w Unraid tworzy je inaczej) zatrzymują przywracanie na miejscu, zanim maszyna zostanie ruszona, bo libvirt nie potrafi usunąć maszyny, która je ma. Najpierw je usuń.

## Tylko zmienione bloki {#changed-blocks}

Maszynę z dyskami qcow2 można kopiować, czytając tylko to, co się zmieniło. Włącz **Tylko zmienione bloki** w tej samej karcie, a BombVault użyje API kopii libvirt zamiast powyższej metody:

- Uruchamia zadanie kopii w trybie pull (`virsh backup-begin`), które w tej samej chwili tworzy punkt kontrolny o nazwie `bombvault-<time>`, i czyta dyski przez łącze SSH, przez przekierowane gniazdo Unix. Gdy agent gościa odpowiada, systemy plików gościa są zamrożone tylko na czas startu zadania. Rozmrożenie, które się nie uda, jest ponawiane przez około piętnaście sekund, a jeśli gość nadal się nie rozmraża, BombVault wysyła powiadomienie.
- Pierwszy przebieg czyta każdy przydzielony blok. Kolejne czytają tylko bloki, które bitmapa punktu kontrolnego oznacza jako zapisane od poprzedniej kopii.
- Każdy dysk jest zapisywany jako folder segmentów o stałym rozmiarze, zawierających to, co widzi gość. Niezmienione segmenty są przejmowane z poprzedniej migawki bez czytania, więc każda migawka jest kompletna i przywraca się sama.
- Przywracanie odbudowuje dysk z jego segmentów i konwertuje go z powrotem do qcow2 przez `qemu-img`. Dysk ma tę samą zawartość co w chwili kopii, ale plik jest nowym plikiem qcow2, a łańcuch bazowy wraca spłaszczony.
- Kontrola przed takim przywracaniem wymienia każdy dysk, który odbudowuje, i liczy go w pełnym rozmiarze gościa, dwukrotnie dla qcow2, bo obraz surowy i jego skonwertowana kopia leżą obok siebie, dopóki stary plik nie zostanie zastąpiony. To najwięcej, ile dysk może zająć, więc niedobór według tego liczenia zostawia linię miejsca szarą zamiast blokować. Sprawdzenie przywracania po pierwszej kopii oraz **Sprawdź przywracanie** upewniają się też, że migawka zawiera każdy segment każdego dysku w rozmiarze, jaki powinien mieć.
- Między przebiegami maszyna nie ma w libvirt zapisanego punktu kontrolnego. BombVault trzyma definicję najnowszego punktu kontrolnego `bombvault-` w `/config/vm-checkpoints` i usuwa tylko zapis o nim w libvirt (`virsh checkpoint-delete --metadata`). Jego bitmapa zostaje w pliku qcow2 i dalej liczy zapisy, a następny przebieg przywraca punkt kontrolny (`virsh checkpoint-create --redefine`) przed startem. libvirt odmawia usunięcia definicji maszyny, która ma punkty kontrolne, więc dzięki temu **Remove VM** w Unraid i przywracanie działają jak zwykle.
- Zadanie kopii, które BombVault zostawił uruchomione, bo zatrzymał się w środku kopii, jest kończone przy następnym starcie BombVault, razem z punktami kontrolnymi, które ten przebieg zostawił w zapisie.
- Wyłączenie przełącznika, wyjęcie maszyny z harmonogramu albo usunięcie jej kopii też kończy takie zadanie i usuwa też bitmapę, gdy maszyna działa. Na wyłączonej maszynie libvirt nie może jej usunąć i zostaje nieużywana w obrazie.
- Punkty kontrolne innych narzędzi są zostawiane w spokoju. Maszyny, która ma takie, nie da się przywrócić, dopóki nie zostaną usunięte, a przywracanie zatrzymuje się, zanim dotknie maszyny.

Przełącznik cofa się sam. Brakujący lub uszkodzony punkt kontrolny, dysk, który zmienił rozmiar, albo najnowsza migawka, która nie jest migawką zmienionych bloków, oznacza, że cały dysk jest czytany jeszcze raz. Wyłączona maszyna, dysk surowy lub zvol, brak łącza SSH z hostem, libvirt bez API kopii albo zbyt mało wolnego miejsca w `/config` oznacza zwykłą kopię powyższą metodą. Karta mówi, której z nich użył ostatni przebieg i dlaczego.

Wymagania: libvirt z API kopii (testowane z libvirt 12.2 i QEMU 10.2 na Unraid 7.3), sshd z `AllowStreamLocalForwarding yes` (domyślnie, także na Unraid, który wyłącza przekierowanie TCP) i dyski qcow2. libvirt odmawia punktów kontrolnych na dyskach surowych, a Unraid domyślnie tworzy surowe vdiski. Maszyna działa podczas kopii, tak jak przy Migawce na żywo, a segmenty czekające na restic zajmują do 4 GiB w `/config`.

## Maszyny z migawkami Unraid {#unraid-snapshots}

Migawka wykonana przez VM Manager w Unraid zamienia dysk qcow2 w łańcuch: oryginalny obraz na dole, a na nim po jednym pliku na migawkę, każdy z tym, co zmieniło się od poprzedniego. Maszyna zapisuje do najnowszego pliku. Dysk z dwiema migawkami to trzy pliki.

BombVault kopiuje taką maszynę w całości:

- **Każdy plik łańcucha.** Podąża za łańcuchem bazowym każdego dysku do końca i kopiuje każdy plik w nim, razem z listą migawek Unraid (`/etc/libvirt/qemu/snapshotdb/<vm>/snapshots.db`, odczytywaną przez SSH). Łańcuch, za którym nie da się dojść do końca, na przykład dlatego, że brakuje jednego pliku w środku, kończy kopię błędem i podaje plik.
- **Kopia na żywo zachowuje migawki.** **Migawka na żywo** kładzie na wierzchu tymczasową nakładkę podczas czytania dysku i potem scala ją z powrotem, tylko do obrazu tuż pod nią. Łańcuch po kopii jest taki sam jak przed nią. Maszyna bez migawek zachowuje się jak zawsze.
- **Przywracanie przywraca łańcuch.** Przywracanie na miejscu zapisuje każdy plik tam, gdzie był, i odkłada listę migawek Unraid, więc VM Manager znów wymienia migawki. Przywracanie obok oryginału zapisuje pliki do folderu kopii i kieruje przywrócone dyski na przywrócone pliki, więc kopia nigdy nie czyta z obrazów oryginału. Lista migawek Unraid wraca tylko przy przywracaniu na miejscu, bo wymienia pliki tam, gdzie je skopiowano. Bez niej kopia startuje tak samo, a VM Manager po prostu nie wymienia dla niej migawek.
- **Tylko zmienione bloki** czyta to, co widzi gość, więc jego przywracanie wraca jako jeden spłaszczony plik qcow2 bez migawek.
- **Przywracanie przywraca dysk z kopii, którą wybierzesz.** Migawki zrobione albo scalone od tej kopii tego nie zmieniają. Kopia sprzed migawki definiuje maszynę na pliku, który zawiera, a kopia sprzed scalenia przywraca swoją nakładkę i działa na niej. Nowsze pliki zostają tam, gdzie są, nieużywane i niezmienione. Kopia, która zawiera nakładkę, ale nie plik pod nią, jest odrzucana, zanim maszyna zostanie ruszona. Definicja maszyny po przywróceniu nie wymienia łańcucha, więc libvirt odczytuje łańcuch z przywróconych plików.

### Przykład: maszyna z dwiema migawkami {#example-unraid-snapshots}

Maszyna `homeassistant` ma jeden dysk, `vdisk1.qcow2`, i dwie migawki, które zrobiłeś w VM Manager przed dwiema aktualizacjami. Jej dysk jest teraz łańcuchem trzech plików. Działa cały czas, więc używa **Migawka na żywo**.

1. Otwórz **Kopie zapasowe**, potem `homeassistant`, a potem jego kartę **Ustawienia**. W karcie **VM** sprawdź, że **Metoda** to **Migawka na żywo**, a **Tylko zmienione bloki** jest wyłączone.
2. Naciśnij **Utwórz kopię teraz** w nagłówku strony.
3. Gdy przebieg się skończy, jego linia w **Ostatnie uruchomienia** na karcie **Weryfikacja** jest zielona. Maszyna działała przez cały czas, a kopia zawiera wszystkie trzy pliki dysku i listę migawek Unraid.
4. W VM Manager obie migawki nadal tam są, a dysk nadal czyta przez te same trzy pliki.

Miesiąc później aktualizacja idzie źle i chcesz mieć wczorajszy stan obok zepsutego, żeby je porównać:

1. Otwórz **Przywracanie**, wybierz **Maszyny wirtualne** i kliknij `homeassistant`.
2. Zostaw zalecaną kopię z ostatniej nocy i w **Dokąd** wybierz **Obok oryginału**.
3. Naciśnij **Przywróć**. Trzy pliki lądują w folderze nazwanym jak maszyna z `-restored`, a przywrócony dysk wskazuje na przywrócone pliki. Działająca maszyna się nie zmienia.

Aby zamiast tego cofnąć całą maszynę, wybierz **Zastąp oryginał** z włączonym **Najpierw zrób kopię obecnego stanu**. BombVault skopiuje zepsuty stan, zamknie maszynę, zapisze z powrotem trzy pliki i listę migawek Unraid, i uruchomi ją ponownie.

## TrueNAS SCALE {#truenas-scale}

Powyższe kroki są pisane dla Unraid, ale to samo łącze `qemu+ssh://` działa z każdym libvirtd, do którego kontener może dotrzeć, także z TrueNAS SCALE, który ma maszyny wirtualne na libvirt znowu od 25.04.2. Na TrueNAS ciąg połączenia wymaga nadpisania, a klucz trafia do roota w ustawieniach użytkowników TrueNAS. Trzy kolejne rzeczy zmieniają zawartość kopii: dyski na zvol, stan vTPM i nazwy, jakie TrueNAS nadaje maszynom.

### 1. Gniazdo libvirt {#truenas-socket}

libvirtd w TrueNAS nie nasłuchuje tam, gdzie oczekuje ciąg połączenia zbudowany z pól **SSH hosta**. Ustaw zamiast tego `LIBVIRT_URI`, z dodatkowym parametrem `?socket=`:

```sh
LIBVIRT_URI=qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock
```

BombVault używa `LIBVIRT_URI` dokładnie tak, jak jest zapisany, a `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` i `LIBVIRT_SSH_PORT` nie są potrzebne. Dla każdej z nich, która nie jest ustawiona, BombVault bierze host, użytkownika i port z URI także dla własnych poleceń SSH, więc kopiowanie NVRAM i kopie zbiorów danych ZFS docierają do tej samej maszyny co `virsh`. Zmienna ustawiona przez Ciebie zawsze wygrywa, a dziennik startowy podaje cel, który wziął z URI. Na TrueNAS użytkownik inny niż root potrzebuje też `zfs allow <user> snapshot,destroy,mount <dataset>` dla kopii zbiorów danych ZFS, zobacz [Zbiory danych ZFS](zfs-datasets.md#truenas).

Wpisz adres TrueNAS w LAN do URI. Gdy nie ustawiono ani `LIBVIRT_URI`, ani `LIBVIRT_HOST`, BombVault bierze domyślną bramę aplikacji jako host, bo `host.docker.internal` nie rozwiązuje się w aplikacji TrueNAS, a dziennik startowy podaje wybrany adres. To wystarcza dla kopii zbiorów danych ZFS. Kopie maszyn wirtualnych nadal potrzebują URI dla gniazda.

Aplikacja potrzebuje uprawnienia `SETPCAP` dla kopii maszyn wirtualnych, obok `CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `SETGID` i `SETUID`. Bez niego libvirt zatrzymuje się z komunikatem „cannot apply process capabilities”. YAML w [TrueNAS SCALE](getting-started.md#truenas) ustawia je wszystkie, razem z `SYS_CHROOT`, którego potrzebuje migawka zamiast zatrzymania.

Na TrueNAS SCALE 25.10.0 ścieżkę gniazda sprawdzono na prawdziwym sprzęcie: domyślne `qemu:///system` kończy się błędem `Failed to connect socket to '/var/run/libvirt/libvirt-sock': No such file or directory`, a to samo wywołanie z `?socket=/run/truenas_libvirt/libvirt-sock` wymienia domeny.

### 2. Klucz BombVault na TrueNAS {#truenas-key}

Krok 2 powyżej zapisuje na dysku flash Unraid i tutaj nie ma zastosowania. Na TrueNAS:

1. Włącz usługę SSH w **System, Services**.
2. W **Credentials, Users** edytuj **root** i wklej klucz publiczny BombVault z **Ustawienia, Integracje, SSH hosta** jako jego publiczny klucz SSH. TrueNAS zachowuje go po restartach i aktualizacjach.

Świeże instalacje TrueNAS 24.10 i nowszych nie dają rootowi hasła, a logowanie kluczem go nie potrzebuje. Jeśli wolisz nie używać roota, autoryzuj klucz dla konta administratora z sudo i wpisz jego nazwę zamiast `root` w `LIBVIRT_URI`. Kopie zbiorów danych ZFS potrzebują wtedy `zfs allow`, jak opisano wyżej.

Replika ZFS na drugi serwer loguje się własnym kluczem, który pokazuje okno repliki. Klucz SSH hosta nie autoryzuje repliki, a klucz repliki nie autoryzuje kopii maszyn wirtualnych.

!!! warning "TrueNAS nie widzi zmian wykonanych przez virsh"
    Warstwa pośrednia TrueNAS, ta za jego ekranami maszyn wirtualnych, ma być jedynym sposobem zmieniania jego maszyn. Polecenie `virsh` przez to łącze SSH, które uruchamia kopia i przywracanie maszyn wirtualnych w BombVault, omija ją, a TrueNAS może później uzgodnić lub nadpisać stan, którego nie utworzył. Wynika to z udokumentowanego zachowania TrueNAS i nie zostało zaobserwowane na prawdziwej maszynie. To ten sam rodzaj ryzyka co uzgadnianie przez VM Manager w Unraid zmian wykonanych poza nim, tylko ostrzejszy na TrueNAS.

### 3. Dyski na zvol {#truenas-zvols}

Dyski maszyn wirtualnych w TrueNAS to często zvol, czyli urządzenia blokowe ZFS pod `/dev/zvol/<pool>/<dataset>`, a nie pliki qcow2. restic nie potrafi kopiować urządzenia blokowego po jego ścieżce tak jak pliku, więc BombVault rozpoznaje dysk zvol w XML domeny (`<source dev="...">` zamiast `<source file="...">`) i obsługuje go przez to samo łącze SSH. Kontener BombVault nie potrzebuje żadnych narzędzi ZFS.

Kopia zvol:

1. `zfs snapshot <dataset>@bombvault-vm-<time>` ustala punkt w czasie, także gdy maszyna działa.
2. `zfs clone -o volmode=dev -o readonly=on` czyni tę migawkę czytelną jako urządzenie. `volmode=dev` powstrzymuje host od skanowania partycji gościa.
3. `dd` czyta urządzenie, a surowy obraz trafia do restic jako jeden plik, `/vm-disks/<dataset>.img`, z tagiem `vm:<name>:zvol:<dev>`. restic tnie go na kawałki według zawartości, więc bloki, które nie zmieniły się od ostatniej kopii, nie są zapisywane ponownie.
4. Klon i migawka są potem niszczone, niezależnie od tego, czy kopia się udała. Jeśli BombVault zatrzymał się w środku kopii, następna kopia tego zvol najpierw usuwa migawkę `bombvault-vm-<time>` i jej klon `<dataset>-bombvault-vm-<time>`. Każdą inną migawkę zostawia w spokoju i zachowuje tę, z której sklonowano coś innego. Dziennik podaje, co usunął lub zachował.

BombVault nie używa do tego `zfs send`. Każdy rekord strumienia send niesie GUID migawki i bieżącą sumę kontrolną, więc restic nie znalazłby nic z ostatniej kopii do ponownego użycia i za każdym razem zapisywałby cały dysk od nowa. Zmierzone na TrueNAS SCALE 25.10 ze zvol o rozmiarze 512 MiB z 200 MiB danych: pierwsza kopia dodała 211 MB, druga bez zmian 52 KB, a po zmianie 1 MiB 3,6 MB. Ten sam zvol przez `zfs send` dodawał 214 MB przy każdym przebiegu.

Każdy przebieg zapisuje też małą migawkę z tagiem `vm:<name>` z XML domeny, gdy wszystkie dyski są już w środku. Maszyna, której wszystkie dyski to zvol, nie ma migawki plikowej, a ta wymienia przebieg pod maszyną i czyni go możliwym do przywrócenia.

Przywracanie zapisuje każdy zvol z powrotem na miejscu, tak jak dysk plikowy wraca do swojego pliku, więc przywrócona maszyna startuje z przywróconych danych:

1. Zanim cokolwiek zostanie dotknięte, BombVault sprawdza, czy przebieg zawiera obraz każdego dysku zvol. Przebieg, któremu któregoś brakuje, jest odrzucany, a maszyna zostaje, jaka jest.
2. Istniejący zvol dostaje najpierw migawkę, `<dataset>@bombvault-before-restore-<time>`, z czasem w UTC jak każda nazwa migawki tworzona przez BombVault, więc może różnić się o godziny od zegara lokalnego. Dziennik ją podaje, a `zfs rollback` do niej cofa przywracanie, dopóki jest najnowszą migawką zvol. Gdy istnieje nowsza, na przykład z następnego przywracania, robi to `zfs rollback -r` i niszczy nowsze migawki. Zostaje, dopóki nie usuniesz jej przez `zfs destroy`, gdy przywrócona maszyna działa.
3. Zvol, którego już nie ma, jest tworzony w rozmiarze, jaki miał w chwili kopii, a istniejący o innym rozmiarze jest ustawiany z powrotem na ten rozmiar.
4. `dd` zapisuje obraz na `/dev/zvol/<dataset>`. Z włączoną kompresją, domyślną w TrueNAS, puste części obrazu wracają jako dziury i nie zajmują miejsca.

**Przywracanie na inną instancję wymaga puli docelowej.** Przenosi dyski plikowe do folderu docelowego, ale folder nie nazywa żadnej puli ZFS, więc bez niej BombVault odrzuca przywracanie, zanim ruszy. Podaj pulę jako parametr `zvolPool` w `POST /api/foreign/restore`: każdy zvol zachowuje swoją ścieżkę pod pulą (`tank/vms/win10/disk0` na `flashpool` staje się `flashpool/vms/win10/disk0`), a XML domeny na nią wskazuje. Okno **Ze starego folderu z kopiami** na stronie Przywracanie nie ma na to pola, więc jest to dostępne tylko przez bezpośrednie wywołanie API. Przywracanie na tej samej instancji zapisuje każdy zvol z powrotem na siebie.

**Eksport jako pliki pomija zvol.** Maszyna, której wszystkie dyski to zvol, jest odrzucana z tym powodem, więc przywróć ją z kopii. Dla maszyny z także dyskami plikowymi eksport zawiera dyski plikowe i uwagę, która wymienia pominięte zvol.

Sprawdzono to na TrueNAS SCALE 25.10 z ZFS 2.3.4: trzy kopie testowego zvol, przywracanie na miejscu po tym, jak zvol został nadpisany i powiększony, i przywracanie na zvol, który wcześniej usunięto. Każde przywracanie odczytało się z tą samą sumą SHA-256 co źródło, zvol miał znowu dawny rozmiar i nie została żadna migawka ani klon. Całą maszynę, której jedynym dyskiem jest zvol, skopiowano działającą, z Graceful i z Live oraz wyłączoną, a przywracanie całej maszyny (undefine, zapis zvol, define, start) też tam zadziałało: zvol odczytał się z sumą kontrolną źródła i maszyna wystartowała.

Dyski plikowe, tak jak używa ich Unraid, nie są niczym z tego dotknięte.

### 4. Stan vTPM {#truenas-vtpm}

Maszyna może mieć wirtualny TPM, którego potrzebują goście z Secure Boot i Windows 11. Jego stan leży poza dyskiem i NVRAM, więc kopia samych tych dwóch rzeczy przywróciłaby gościa z nowym, pustym TPM.

- **TPM przekazany (passthrough)**, prawdziwy układ przekazany gościowi, podaje swoją ścieżkę w XML domeny. BombVault zapisuje jego stan obok NVRAM przez SSH i odkłada przed `virsh define`, w trybie best effort jak NVRAM: nieudany odczyt lub zapis jest logowany i nie psuje kopii ani przywracania.
- **Emulowany vTPM**, taki, jaki TrueNAS ustawia dla gości Windows 11 i Secure Boot, nie publikuje w XML domeny ścieżki stanu, więc jego stan nie trafia do kopii. BombVault nie zgaduje ścieżki, a kopia kończy się uwagą, że nie ma w niej stanu TPM maszyny. Miej pod ręką klucz odzyskiwania takiego gościa, na przykład klucz BitLocker, zanim go przywrócisz.
- **Nie możesz sam dołączyć TPM do maszyny TrueNAS.** API urządzeń maszyn wirtualnych w TrueNAS SCALE 25.10 przyjmuje tylko `CDROM`, `DISPLAY`, `NIC`, `PCI`, `RAW`, `DISK` i `USB`, a `TPM` odrzuca jako nieznany typ.

Obsługa TPM jest przetestowana tylko jednostkowo i nie była uruchamiana na prawdziwym gościu TrueNAS z vTPM. Maszyna bez TPM kopiuje się i przywraca dokładnie jak dotąd.

NVRAM jest zapisywany jak zwykle. TrueNAS trzyma go dla każdej maszyny w `/var/db/system/vm/nvram/<id>_<name>_VARS.fd`, a XML domeny podaje tę ścieżkę.

### 5. Nazwy maszyn na TrueNAS {#truenas-names}

Na Unraid nazwa maszyny w libvirt to nazwa, którą wybrałeś. TrueNAS wymyśla nazwę libvirt i zmienił to dwa razy:

| Wersja TrueNAS | Nazwa libvirt | Gdzie leży Twoja nazwa |
|---|---|---|
| 25.10 „Goldeye” | `{id}_{name}`, na przykład `1_debian` | w nazwie libvirt, za `{id}_` |
| 26 | UUID maszyny, na przykład `550e8400-e29b-41d4-a716-446655440000` | tylko w `<title>` w XML domeny |

Na hoście, który BombVault rozpoznaje jako TrueNAS albo który nazywa `PLATFORM=truenas`, lista Kopie zapasowe pokazuje Twoją nazwę: `debian` zamiast `1_debian` na 25.10 oraz `<title>` zamiast UUID na 26, albo UUID, gdy nie ma tytułu. To tylko to, co pokazuje lista. Każde wywołanie `virsh` i ustawienia maszyny nadal używają nazwy libvirt, więc kopia i przywracanie zawsze działają na właściwej maszynie. Na TrueNAS 26 oznacza to, że maszyna, którą TrueNAS tworzy ponownie z nowym UUID, pojawia się jako nowy element bez swoich ustawień, choć jej nazwa wygląda tak samo. Na Unraid nic się nie zmienia, także dla maszyny, którą nazwałeś `10_Windows`.

Schemat z 25.10 zaobserwowano na prawdziwej maszynie z TrueNAS SCALE 25.10. Schemat z TrueNAS 26 pochodzi z dokumentacji TrueNAS i jest przetestowany tylko jednostkowo.

## Rozwiązywanie problemów {#troubleshooting}

| Objaw | Przyczyna i rozwiązanie |
|---|---|
| **Testuj połączenie** zawiesza się, potem kończy błędem | Hosta nie da się osiągnąć. Sprawdź krok 3 (sieć, VLAN, zapora) oraz **Host SSH: Address** i **Port**. |
| `Permission denied (publickey)`, choć klucz jest w `authorized_keys` | `StrictModes` w sshd odrzuca plik klucza, bo nadrzędny folder ma złego właściciela albo uprawnienia. Zdarza się to często, gdy `/root/.ssh` jest linkiem do dysku flash FAT (`/boot/config/ssh/...`), który jest zapisywalny dla wszystkich. Dziennik hosta pokazuje `Authentication refused: bad ownership or modes for directory ...`. Dodaj `StrictModes no` do konfiguracji sshd hosta w sekcji globalnej, przed jakimkolwiek blokiem `Match`, potem uruchom `/etc/rc.d/rc.sshd restart`. Unraid zapisuje `/etc/ssh/sshd_config` od nowa przy każdym uruchomieniu, więc utrwal to, jak opisano niżej. |
| `Permission denied (publickey)` i klucza naprawdę brakuje | Powtórz krok 2 i sprawdź, czy SSH jest włączone. |
| `Host key verification failed` | Uruchom `docker exec BombVault rm -f /config/ssh/known_hosts` i przetestuj ponownie. |
| Sprawdzenie z kroku 3 mówi `UNREACHABLE` | Kontener nie dociera do portu SSH hosta. Zobacz krok 3: dostęp hosta do sieci niestandardowych, routing VLAN albo adres shim lub adres w LAN. |
| Brakuje zmiennych pod **Edit** | Zaimportuj szablon ponownie. Istniejący kontener zachowuje swoją starą zapisaną konfigurację. |
| Maszyna z migawkami Unraid kończy się błędem i podaje plik | Łańcucha bazowego dysku nie da się prześledzić do końca. Sprawdź w VM Manager, czy każda migawka tego dysku nadal istnieje i czy wskazany plik leży pod `/mnt`. |

Więcej objawów dotyczących maszyn wirtualnych jest w [Rozwiązywaniu problemów](troubleshooting.md#vm-backup-will-not-connect).

## Utrwalanie po restartach {#persistence}

Unraid zapisuje `/etc/ssh/sshd_config` od nowa przy każdym uruchomieniu, więc zmiany po stronie hosta trzeba wykonywać ponownie przy starcie:

- **Port SSH** jest trzymany w `/boot/config/ident.cfg`, ustawiany przez interfejs webowy Unraid.
- **`StrictModes no`**, jeśli było potrzebne powyżej, nie jest zachowywane. Zastosuj je przy starcie z `/boot/config/go`:

  ```sh
  # /boot/config/go: keep SSH key auth working with flash-based authorized_keys
  ( for i in $(seq 1 30); do [ -f /etc/ssh/sshd_config ] && break; sleep 2; done
    grep -q '^StrictModes no' /etc/ssh/sshd_config || \
      sed -i '0,/^Match /s/^Match /StrictModes no\n&/' /etc/ssh/sshd_config
    # SIGHUP reloads sshd's config in place: it keeps the listener + open sessions
    # and avoids the "killing listener process / Restarting SSH server daemon"
    # warning that `rc.sshd restart` prints at boot. Only reload if sshd is already
    # up; if it starts after this runs, it reads the edited config on its own.
    pidof sshd >/dev/null && killall -HUP sshd ) &
  ```

- Własny klucz SSH BombVault i `~/.ssh/config` leżą w `/config` (appdata) i są tworzone od nowa przy każdym starcie, więc przetrwają restarty kontenera i hosta.

## Bezpieczeństwo {#security}

Autoryzowany klucz daje roota na hoście, ten sam poziom zaufania co gniazdo Docker, którego BombVault już używa. Trzymaj BombVault w zaufanej sieci. Klucz jest ed25519, zapisany z uprawnieniami `0600` w appdata, a klucz hosta jest przypięty. Każde polecenie hosta jest przekazywane jako osobne argumenty, nigdy przez zdalną powłokę, a połączenie SSH używa `BatchMode` i limitu czasu połączenia.
