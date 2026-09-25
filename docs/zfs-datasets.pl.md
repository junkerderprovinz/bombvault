# Zbiory danych ZFS

Strona **ZFS** tworzy kopie zapasowe zbiorów danych ZFS. Element to jeden zbiór danych razem ze wszystkimi zbiorami danych pod nim. Przy każdej kopii BombVault robi jedną migawkę ZFS całego drzewa, więc każdy zbiór danych w nim zostaje uchwycony w tej samej chwili. Następnie odczytuje pliki każdego zbioru danych z tej migawki, zapisuje je za pomocą restic tak samo jak folder i od razu potem usuwa migawkę. Kopie są deduplikowane, każdą z nich możesz przeglądać, a pojedyncze pliki da się przywrócić.

BombVault nigdy nie używa `zfs send` dla zbiorów danych, nigdy nie cofa zbioru danych do wcześniejszego stanu i nigdy żadnego nie niszczy.

## Wymagania {#requirements}

- **Połączenie SSH z tym serwerem.** Zbiory danych ZFS korzystają z tego samego klucza, hosta i użytkownika co kopie VM. Jeśli kopie VM już działają, to też zadziała. W przeciwnym razie skorzystaj z [przewodnika Kopia VM przez SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) na GitHubie. Pola szablonu nazywają się **Host SSH: Address**, **Host SSH: Port** i **Host SSH: User**.
- **Polecenie `zfs` na tym hoście.** Unraid 6.12 i nowszy oraz TrueNAS SCALE je mają.
- **Host Data zmapowane jako `/mnt` z Access Mode Read/Write - Slave.** To domyślne ustawienie szablonu. Migawka zbioru danych pojawia się w jego folderze `.zfs/snapshot` dopiero po uruchomieniu BombVault, więc kontener musi otrzymywać montowania, które host wykonuje później.
- **Zbiory danych zamontowane pod `/mnt`.** W Unraid pule znajdują się pod `/mnt/<pool>`, więc tak już jest.

Włącz domenę w **Ustawienia, Ogólne** (Zbiory danych ZFS). Strona ZFS pokaże wtedy kartę **Połączenie z tym serwerem**. Testuje ona połączenie SSH, podaje użytkownika i host, z którymi się łączy, i mówi, czego brakuje, gdy czegoś brakuje. Sprawdzenie integracji z hostem (`/spike`) pokazuje ten sam wynik.

## Elementy i zbiory podrzędne {#items-and-children}

Otwórz **Dodaj zbiory danych** na stronie ZFS. Lista pochodzi z serwera. Wybierz zbiór danych położony najwyżej w tym, co chcesz kopiować, na przykład `cache/appdata`, a element obejmie go i każdy zbiór danych pod nim.

- **Nowe zbiory podrzędne dołączają same.** Zbiór danych utworzony później pod elementem jest kopiowany w następnym przebiegu, który oznacza go jako nowy. Jego pierwsza kopia odczytuje go raz w całości; potem odczytywane są tylko zmiany.
- **Możesz pominąć pojedyncze zbiory podrzędne.** Wyłącz jeden w ustawieniach elementu, a zostanie pominięty razem ze wszystkim pod nim. Pominięty zbiór podrzędny, którego nie ma już na serwerze, jest tak oznaczony i można go usunąć z listy.
- **Zbiory podrzędne, których nie da się odczytać, są pomijane, nigdy po cichu.** Przebieg je wymienia, element pokazuje, ile pominięto, a karta pokrycia na panelu liczy każdy z nich jako niechroniony. Przebieg i tak kopiuje całą resztę i nie kończy się błędem z powodu pominiętego zbioru. Powody znajdziesz w [tabeli kodów przyczyn](#reason-codes): zbiór danych niezamontowany, z `canmount=off`, z punktem montowania `legacy` lub bez niego, z niezaładowanym kluczem szyfrowania, z wyłączonym dostępem do migawek albo z punktem montowania, którego BombVault nie widzi.
- **Pominięty zbiór danych nie pociąga za sobą swoich zbiorów podrzędnych.** Zbiór z `canmount=off`, który zawiera tylko inne zbiory danych, jest pomijany (pokazywany jako "tylko struktura"), a jego zamontowane zbiory podrzędne są kopiowane. Zaszyfrowany zbiór danych z niezaładowanym kluczem jest pomijany razem ze zbiorami podrzędnymi, które dzielą jego klucz.
- **Zbiory podrzędne będące dyskami VM lub danymi systemowymi startują wyłączone** w oknie dodawania, z powodem obok przełącznika. Dodanie całej puli wymaga potwierdzenia, które wymienia, co ona zawiera.

### Wolumeny {#volumes}

Wolumen (zvol) zawiera dysk wirtualny zamiast plików, a strona ZFS nigdy go nie kopiuje.

- Wolumen używany przez VM jest kopiowany razem z tą VM na stronie **Maszyny wirtualne**.
- Wolumen, którego nie używa żadna VM (extent iSCSI, odłączony dysk), **nie jest kopiowany przez BombVault**. Okno dodawania i strona ZFS liczą takie wolumeny i o tym informują. Przyszła wersja będzie je kopiować.

Wolumeny w drzewie elementu są pomijane i wymieniane przy każdym przebiegu.

### Magazyn Dockera {#docker-storage}

Przy sterowniku magazynu ZFS w Dockerze każda warstwa obrazu jest zbiorem danych z punktem montowania `legacy`. Okno dodawania zwija je do jednego wiersza na rodzica. Drzewo zawierające ponad 20 takich zbiorów nie może zostać elementem: dopóki istnieje jego migawka, Docker nie może usuwać warstw obrazów. Zamiast tego dodaj zbiory danych pod nim, na przykład `appdata`.

### Elementy nigdy się nie nakładają {#overlap}

Zbiór danych może należeć tylko do jednego elementu. BombVault odrzuca nowy element, który leży wewnątrz istniejącego lub który by go zawierał. Aby połączyć kilka elementów podrzędnych w jeden element nadrzędny, usuń najpierw elementy podrzędne, wybierając zachowanie ich kopii, a potem dodaj element nadrzędny. Każdy zbiór danych zachowuje historię pod własną nazwą, więc następna kopia zaczyna tam, gdzie skończyły stare elementy, i nie odczytuje wszystkiego od nowa.

## Zatrzymywanie kontenerów i polecenia wokół migawki {#consistency}

Migawka działającej bazy danych jest jak nagła przerwa w zasilaniu: baza zwykle się podnosi, ale musi to zrobić. Każdy element może zrobić w tej sprawie dwie rzeczy, a obie dotyczą tylko chwili migawki, nie całej kopii.

- **Zatrzymaj te kontenery na czas migawki.** BombVault zatrzymuje wymienione kontenery, robi migawkę i od razu uruchamia je ponownie. Kontenery jednego poziomu zależności zatrzymują się równolegle, najpierw zależne, więc całe okno trwa zwykle kilka sekund; przebieg pokazuje, jak długo. Kopia odczytuje potem zamrożoną migawkę, gdy aplikacje już znowu działają. Zatrzymywane są tylko kontenery, które działały.
- **Polecenie przed migawką i po niej.** Uruchamia się w wybranym przez ciebie kontenerze, na przykład żeby zrzucić bazę danych do zbioru danych tuż przed migawką, niczego nie zatrzymując. Jeśli polecenie przed migawką się nie powiedzie, kopia kończy się błędem i migawka nie powstaje. Nieudane polecenie po migawce jest pokazywane przy przebiegu, ale nie powoduje błędu kopii.

Co się dzieje, gdy coś pójdzie nie tak:

- Jeśli kontenera nie da się zatrzymać, BombVault uruchamia te, które już zatrzymał, a kopia kończy się błędem z nazwą kontenera. Nigdy nie wraca do migawki działających aplikacji.
- Zatrzymanie czeka na zakończenie trwającej kopii kontenera (do 30 minut przy ręcznym przebiegu, do limitu czasu kopii przy zaplanowanym), żeby oba nigdy jednocześnie nie zatrzymywały i nie uruchamiały tego samego kontenera.
- Zanim zatrzyma się pierwszy kontener, BombVault zapisuje, które zatrzymuje. Jeśli BombVault zostanie zabity w trakcie okna, przy następnym starcie uruchomi te kontenery ponownie, wyśle powiadomienie, a element pokaże czerwoną notatkę przy każdym kontenerze, którego nie udało się uruchomić.

Automatyczne zrzuty baz danych (zob. [Funkcje](features.md)) działają razem z własną kopią kontenera na stronie **Containers**, a nie z elementem ZFS. Baza danych, której kontener jest kopiowany tylko przez jego zbiór danych, nie dostaje zrzutu, więc daj jej tutaj polecenie.

Kontener może być jednocześnie na tej liście i na stronie **Containers**. Jego dane są wtedy zapisywane dwa razy, w dwóch repozytoriach, a **Pełna kopia** zatrzymuje go dwa razy. Element o tym informuje.

## Przywracanie {#restore}

Otwórz **Kopie zapasowe** przy elemencie, wybierz kopię, a potem zbiór danych. Domyślnie jest to najwyższy zbiór danych elementu.

- **Przywróć do samego zbioru danych.** Pliki z kopii są zapisywane w punkcie montowania zbioru danych. Pliki o tej samej nazwie są nadpisywane, pozostałe zostają. Zbiór danych nigdy nie jest cofany ani zastępowany. BombVault sprawdza, czy zbiór danych jest zamontowany, widoczny i zapisywalny, raz przed rozpoczęciem i ponownie tuż przed zapisem. Tam, gdzie wewnątrz zamontowany jest zbiór podrzędny, nic nie jest zapisywane: zbiór podrzędny zachowuje swoje pliki, właściciela i uprawnienia i jest przywracany z własnej kopii.
- **Przywróć do folderu.** Wybierz folder pod `/mnt`. BombVault sprawdza, czy folder leży na zamontowanej puli lub udziale i czy jest dość wolnego miejsca. Działa to bez połączenia SSH i dla zbiorów danych, które już nie istnieją.
- **Wybierz pliki** (zaawansowane): zapisz z powrotem w zbiorze danych tylko wybrane pliki i foldery.
- **Wszystkie zbiory danych tej kopii** (zaawansowane): każdy zbiór danych drzewa do własnego podfolderu wybranego folderu. Zbiory danych pominięte w tej kopii są wymieniane.
- **Z innego serwera:** strona **Odzyskiwanie** przywraca z repozytorium innego BombVault, zawsze do folderu: wszystkie zbiory danych jednej kopii, każdy do własnego podfolderu, albo jeden zbiór danych drzewa, w całości lub wybrane pliki.

Lista kontenerów do zatrzymania z elementu jest proponowana także przy przywracaniu do zbioru danych. Te kontenery pozostają zatrzymane przez całe przywracanie, a kopie kontenerów w tym czasie czekają.

### Migawka bezpieczeństwa {#safety-snapshot}

Zanim zapisze coś w zbiorze danych, BombVault robi migawkę ZFS tylko tego zbioru danych o nazwie `bombvault-prerestore-<czas>`. Jest włączona domyślnie; jej wyłączenie wymaga drugiego potwierdzenia. Jeśli migawki nie da się zrobić, nic nie jest przywracane.

BombVault nigdy sam nie usuwa migawki bezpieczeństwa. Element wymienia je z wiekiem i rozmiarem, każdą z akcją **Usuń**, i ostrzega, gdy najstarsza ma więcej niż 30 dni, bo trzyma na puli usunięte i zmienione dane.

Aby wrócić po przywróceniu, skopiuj pojedyncze pliki z `.zfs/snapshot/bombvault-prerestore-<czas>` w zbiorze danych. `zfs rollback <dataset>@bombvault-prerestore-<czas>` działa tylko, dopóki jest to najnowsza migawka tego zbioru danych. `zfs rollback -r` usuwa każdą nowszą migawkę, łącznie z automatycznymi.

### Przywracanie jako nowy zbiór danych {#new-dataset}

BombVault nie tworzy zbiorów danych. Utwórz go na serwerze z żądanymi właściwościami, a potem przywróć do folderu, który jest jego punktem montowania:

```
zfs create -o compression=lz4 cache/appdata-restored
```

a w BombVault przywróć do folderu `cache/appdata-restored` pod `/mnt`.

## Co jest w kopii {#contents}

W kopii: pliki i foldery każdego skopiowanego zbioru danych, z właścicielem, uprawnieniami, znacznikami czasu i atrybutami rozszerzonymi, tak jak zapisuje je restic.

Poza kopią:

- właściwości ZFS zbiorów danych (compression, recordsize, quota, mountpoint i pozostałe);
- właściciel i uprawnienia samego najwyższego folderu każdego zbioru danych (wszystko pod nim jest uwzględnione). Przywrócenie do zbioru danych zostawia istniejący najwyższy folder bez zmian, przywrócenie do folderu tworzy go z uprawnieniami `0755`;
- istniejące migawki ZFS;
- zbiory podrzędne, które zostały pominięte lub wyłączone;
- wolumeny.

Aby przywrócić na nową pulę, najpierw utwórz zbiory danych z żądanymi właściwościami. Nie sprawdzono jeszcze, czy listy ACL NFSv4, których TrueNAS używa na zbiorach danych SMB, wracają tak, jak oczekujesz, więc przetestuj przywracanie na własnych danych, zanim na nich polegasz.

## Zaszyfrowane zbiory danych {#encryption}

Zaszyfrowany zbiór danych jest kopiowany tylko wtedy, gdy jego klucz jest załadowany. W przeciwnym razie jest pomijany z ostrzeżeniem; załaduj klucz poleceniem `zfs load-key` i zamontuj zbiór danych. BombVault odczytuje dane odszyfrowane i zapisuje je w repozytorium restic, które jest zaszyfrowane. Jeśli wyłączyłeś szyfrowanie w BombVault, to repozytorium nie jest zaszyfrowane.

## Pozostałe migawki {#leftover-snapshots}

Migawka kopii nazywa się `<dataset>@bombvault-<14 cyfr>`, na przykład `cache/appdata@bombvault-20260924021500` (UTC). BombVault usuwa ją zaraz po kopii. Jeśli to się nie uda, na przykład dlatego, że zbiór danych jest zajęty albo BombVault został zatrzymany, BombVault usuwa ją:

- przed następną kopią tego elementu,
- przy starcie BombVault, dla każdego elementu, także przy wyłączonej domenie,
- gdy usuwasz element,
- gdy naciśniesz **Usuń teraz** przy elemencie, który pokazuje też, ile ich zostało.

Usuwane są tylko nazwy, które dokładnie odpowiadają `bombvault-` plus 14 cyfr. Migawki bezpieczeństwa, twoje własne migawki i migawki automatyczne nigdy nie są ruszane. Aby usunąć jedną ręcznie:

```
zfs destroy -r cache/appdata@bombvault-20260924021500
```

## Anomalie {#anomalies}

Zbiór podrzędny, który został opróżniony, ledwo zmienia sumę dużego drzewa, dlatego wykrywanie anomalii obserwuje każdy zbiór danych elementu osobno: jego rozmiar, liczba plików, nowe dane i czas restic mają każdy własną historię. Ta historia należy do nazwy zbioru danych, więc zostaje, gdy drzewo jest później kopiowane przez inny element.

Zbiór danych, który poprzedni przebieg skopiował, a którego ten przebieg nie mógł odczytać, liczy się jako opróżniony, o ile wybór elementu się nie zmienił. Obejmuje to niezaładowany klucz, niezamontowany zbiór danych i taki, który zniknął z drzewa. Zbiór podrzędny, który sam wykluczysz, zmienia wybór, więc jego historia zaczyna się od nowa. Dopóki otwarte jest ustalenie o utraconych danych, retencja zachowuje stare kopie tylko tego zbioru danych, a resztę drzewa przycina jak zwykle.

Na karcie **Elementy** strony **Anomalie** każdy zbiór danych ma własny wiersz pod swoim elementem, a drzewo elementu na tej stronie pokazuje otwarte ustalenia obok każdego zbioru danych. Link w ustaleniu otwiera panel przywracania elementu na ostatniej dobrej kopii zbioru danych. To, czy przebieg dobiega końca, ocenia się dla całego elementu, bo przebieg udaje się albo nie jako całość.

Same kontrole opisano w [Funkcje](features.md). Asystent połączony przez [serwer MCP](mcp.md) może wypisać punkty przywracania elementu ZFS, uruchomić jego kopię i czytać ustalenia, ale potwierdzenie ustalenia odbywa się na stronie **Anomalie**.

## Kody przyczyn {#reason-codes}

Strona, historia przebiegów i powiadomienia nazywają problem jednym z tych kodów. Przy większości rozwiązanie jest też widoczne obok na stronie.

| Kod | Znaczenie | Co zrobić |
|---|---|---|
| `ssh-missing` | Połączenie SSH nie jest skonfigurowane w tym kontenerze. | Skonfiguruj połączenie SSH tak jak dla kopii VM. |
| `host-placeholder` | Host SSH: Address to wciąż wartość przykładowa, a `host.docker.internal` też nie odpowiedział. | Ustaw Host SSH: Address na adres IP LAN tego serwera. |
| `host-fallback` | Host SSH: Address to wciąż wartość przykładowa, a `host.docker.internal` działa. | Nic, albo ustaw adres IP LAN. |
| `ssh-unreachable` | Serwer jest nieosiągalny przez SSH. | Sprawdź adres i port oraz czy SSH jest włączone. |
| `ssh-auth` | Serwer odrzucił klucz BombVault. | Uruchom raz na serwerze polecenie pokazane na karcie połączenia. |
| `zfs-not-found` | Host SSH nie ma polecenia `zfs`. | Skieruj Host SSH: Address na maszynę, do której należą pule. |
| `zfs-permission` | Użytkownik SSH nie może uruchomić tego polecenia zfs. | Użyj roota albo zob. [TrueNAS SCALE](#truenas). |
| `uri-mismatch` | `LIBVIRT_URI` wskazuje inny host lub użytkownika niż pola SSH. | Uzgodnij je albo wyczyść pola SSH, żeby oba pochodziły z URI. |
| `zfs-error` | zfs zgłosił inny błąd. | Szczegóły pokazują jego komunikat. |
| `propagation-missing` | Nowe montowania na hoście nie docierają do kontenera. | Ustaw Access Mode dla Host Data na Read/Write - Slave i uruchom BombVault ponownie. |
| `invalid-name` | Nazwa zbioru danych, której BombVault nie przyjmuje. | Zmień nazwę zbioru danych. |
| `name-too-long` | Zbiór danych w drzewie jest za długi na nazwę migawki. | Zmień jego nazwę albo dodaj jako element zbiór danych pod nim. |
| `invalid-exclude` | Wzorzec wykluczenia lub pominięty zbiór podrzędny nie pasuje do elementu. | Popraw wpis wskazany w komunikacie. Aby pominąć cały zbiór podrzędny, wyłącz go zamiast pisać wzorzec. |
| `not-found` | Zbiór danych nie istnieje na serwerze. | Usuń element albo utwórz zbiór danych ponownie. Jego kopie nadal można przywrócić. |
| `not-filesystem` | To jest wolumen, nie system plików. | Zob. [Wolumeny](#volumes). |
| `overlaps-item` | Zbiór danych nakłada się na istniejący element. | Zob. [Elementy nigdy się nie nakładają](#overlap). |
| `docker-storage` | Drzewo zawiera magazyn obrazów Dockera. | Zob. [Magazyn Dockera](#docker-storage). |
| `nothing-readable` | Żadnego zbioru danych elementu nie da się teraz odczytać. | Sprawdź kody pominiętych zbiorów danych. |
| `snapshot-failed` | Nie udało się utworzyć migawki. | Szczegóły pokazują komunikat zfs. |
| `containers-busy` | Kopia kontenera wciąż trwała, gdy kontenery miały się zatrzymać. | Uruchom ponownie później. Zaplanowane przebiegi czekają same. |
| `consistency-stop-failed` | Kontenera nie dało się zatrzymać, więc migawka nie powstała. | Sprawdź kontener albo usuń go z listy. |
| `pre-snapshot-failed` | Polecenie przed migawką się nie powiodło. | Szczegóły przebiegu pokazują jego wyjście. |
| `container-unknown` | Wymieniony kontener nie istnieje. | Usuń go z listy. |
| `container-is-self` | BombVault nie może zatrzymać własnego kontenera. | Usuń go z listy. |
| `leftover-snapshots` | Na serwerze wciąż są migawki, których BombVault nie mógł usunąć. | Naciśnij **Usuń teraz**, zob. [Pozostałe migawki](#leftover-snapshots). |
| `zvol` | Wolumen w drzewie, pominięty. | Zob. [Wolumeny](#volumes). |
| `canmount-off` | Nigdy nie montowany (`canmount=off`), pominięty. | Jeśli zawiera dane, zamontuj go albo przenieś dane do zbioru podrzędnego. |
| `legacy-mount` | Punkt montowania legacy, pominięty. | Nadaj mu punkt montowania pod `/mnt`. |
| `no-mountpoint` | Brak punktu montowania, pominięty. | Nadaj mu punkt montowania pod `/mnt`. |
| `not-mounted` | Niezamontowany na serwerze, pominięty. | Zamontuj go poleceniem `zfs mount` albo ustaw `canmount=on`. |
| `key-not-loaded` | Zaszyfrowany, a klucz nie jest załadowany, pominięty. | `zfs load-key`, potem go zamontuj. |
| `snapdir-disabled` | Dostęp do migawek jest wyłączony, pominięty. | `zfs set snapdir=hidden <dataset>`. Folder `.zfs` pozostaje ukryty. |
| `not-visible` | BombVault nie widzi punktu montowania zbioru danych. | Przenieś punkt montowania pod ścieżkę Host Data albo zmapuj go do kontenera pod tą samą ścieżką z Read/Write - Slave. |
| `shfs-only` | Zbiór danych jest widoczny tylko przez `/mnt/user`, które ukrywa migawki. | Zmapuj jako Host Data `/mnt`, nie `/mnt/user`. |
| `snapshot-not-visible` | Migawka powstała, ale nie pojawiła się w BombVault. | Uruchom **Sprawdź dostęp do migawek**; zob. niżej. |
| `snapshot-loop` | Migawka nie dotarła do BombVault, bo Host Data nie przepuszcza nowych montowań. | Ustaw Access Mode dla Host Data na Read/Write - Slave i uruchom BombVault ponownie. |
| `backup-failed` | restic nie powiódł się dla tego zbioru danych. | Szczegóły przebiegu pokazują dlaczego. |
| `not-reached` | Przebieg skończył się przed tym zbiorem danych. | Uruchom kopię ponownie. |
| `gone` | Zbioru danych nie ma już na serwerze. | Nic. Jego kopie nadal można przywrócić. |
| `read-only-mount` | BombVault może tylko czytać zbiór danych, więc nie może do niego przywracać. | Ustaw mapowanie na Read/Write - Slave albo przywróć do folderu. |
| `destination-not-mounted` | Folder nie leży na zamontowanej puli ani udziale. | Wybierz folder na puli lub udziale. |
| `not-enough-space` | Za mało wolnego miejsca w miejscu docelowym. | Zwolnij miejsce albo wybierz inny folder. |
| `safety-snapshot-failed` | Nie udało się zrobić migawki bezpieczeństwa, więc nic nie przywrócono. | Szczegóły pokazują komunikat zfs. |
| `safety-name-too-long` | Nazwa zbioru danych jest za długa na migawkę bezpieczeństwa. | Wyłącz migawkę bezpieczeństwa albo przywróć do folderu. |

### Sprawdzanie, co widzi kontener {#mountinfo}

**Sprawdź dostęp do migawek** przy elemencie robi prawdziwą migawkę jego drzewa, szuka jej w BombVault dla każdego zbioru danych i ją usuwa. To najszybszy sposób, by sprawdzić całą ścieżkę przed pierwszym zaplanowanym przebiegiem.

Aby sprawdzić samodzielnie, uruchom to na serwerze:

```
docker exec BombVault grep zfs /proc/self/mountinfo
```

Każdy wiersz to jedno montowanie w kontenerze. Wiersz zbioru danych pokazuje jego ścieżkę w kontenerze (pod `/host/user`) i nazwę zbioru danych. Pole `master:N` w tym wierszu oznacza, że montowanie otrzymuje montowania wykonywane później przez host, a tego potrzebuje dostęp do migawek. Jeśli go brakuje, ustaw Access Mode dla Host Data na Read/Write - Slave i uruchom BombVault ponownie.

## TrueNAS SCALE {#truenas}

- Gdy ustawiono `LIBVIRT_URI` (jak dla kopii VM na TrueNAS), BombVault bierze z URI host, użytkownika i port SSH dla swoich poleceń zfs, każdy, który nie jest ustawiony osobno. Bez kopii VM ustaw zamiast tego `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` i `LIBVIRT_SSH_PORT`. Dodaj zmienne w **Additional Environment Variables**.
- Użytkownik inny niż root potrzebuje uprawnień do najwyższego zbioru danych elementu, które obejmują wtedy każdy zbiór danych pod nim:

  ```
  zfs allow <user> snapshot,destroy,mount <dataset>
  ```

  Sesja SSH bez roota na TrueNAS nie ma `/usr/sbin` w ścieżce; BombVault wywołuje wtedy bezpośrednio `/usr/sbin/zfs`.
- **Host Data** aplikacji musi być ścieżką hosta powyżej zbiorów danych, na przykład `/mnt/tank`, a nie ixVolume. Ze ścieżką hosta aplikacja przekazuje nowe montowania hosta do BombVault (`rslave`), a tego potrzebuje dostęp do migawek.
