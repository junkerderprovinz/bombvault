# Miejsca przechowywania

Miejsce przechowywania to miejsce, w którym BombVault trzyma kopie zapasowe: folder na tym serwerze Unraid, udział na NAS, zasobnik u dostawcy chmury, rest-server, konto SFTP albo Nextcloud. Każde miejsce podłączasz raz, w **Ustawienia, Magazyn**, a jego poświadczenia, przechowywanie, ochrona i lokalizacja należą do niego. Domeny (kontenery, VM, flash, własna konfiguracja BombVault, zestawy plików i zbiory danych ZFS) wybierają potem spośród miejsc: gdzie każda domena jest przechowywana i dokąd jest kopiowana. Zbiory danych ZFS mają swój wiersz na karcie Domeny, dopóki domena ZFS jest włączona.

## Dodawanie miejsca {#add-a-place}

**Dodaj miejsce** otwiera okno z jednym kafelkiem na dostawcę, w trzech grupach: magazyn w chmurze, własny hosting oraz NAS i ten serwer.

1. Wybierz kafelek i wypełnij jego formularz. Przycisk z okiem pokazuje wpisany sekret.
2. **Testuj połączenie** sprawdza miejsce i niczego nie tworzy. Dla folderu każdej domeny podaje, co zastał: folder pusty albo jeszcze nieistniejący, folder z repozytorium restic albo błąd, który zatrzymał test.
3. Nadaj miejscu nazwę; nazwa dostawcy jest już wpisana. Dla urządzenia, które prowadzisz sam, odpowiedz na pytanie **Gdzie stoi urządzenie?**. Dostawcy chmury są zawsze w innej lokalizacji, a folder na tym serwerze Unraid jest zawsze tutaj.
4. **Dodaj** zapisuje miejsce.

Nowe miejsce nie jest jeszcze używane przez żadną domenę. Wybierz je pod **Przechowywane w** lub **Kopiowane do** na [karcie Domeny](#domains) albo na karcie elementu, tylko dla tego elementu.

## Foldery {#folders}

Miejsce trzyma jeden folder na domenę: `container`, `vms`, `flash`, `config`, `files` i `zfs`, czyli nazwy, których używają domyślne lokalizacje kopii zapasowych. Foldery są wymienione w szczegółach miejsca i tam można zmienić ich nazwy (zobacz [Zmiana adresu](#addresses)). Domena, która nie ma folderu w danym miejscu, nie może go wybrać.

Gdy domena używa miejsca w obu rolach, druga rola dostaje przyrostek, a pierwsza zachowuje swój folder. Miejsce, które już otrzymuje kopie domeny, trzyma elementy wysyłane wprost do niego w `<folder>-direct`; miejsce, które już przechowuje domenę, otrzymuje jej kopie w `<folder>-copies`.

Niektóre miejsca same są repozytorium restic: adres, pod którym było już repozytorium, gdy miejsce dodawano, nazwane repozytorium z istniejącej konfiguracji albo cel kopii w katalogu głównym zasobnika. Takie miejsce nie ma folderów, wszystkie domeny dzielą jego jedno repozytorium i nie przyjmuje ono drugiej roli. Aby przechowywać więcej u tego samego dostawcy, podłącz inny zasobnik lub folder jako osobne miejsce.

## Szczegóły miejsca {#details}

Każde miejsce to wiersz z dostawcą, informacją, do czego jest używane, oraz ostatnim testem lub ostatnią kopią. **Testuj** sprawdza każdy adres w tym miejscu, a **Szczegóły** otwierają jego ustawienia. Każda zmiana w szczegółach zapisuje się od razu.

- **Ogólne**: nazwa, przełącznik, który włącza i wyłącza miejsce, adres oraz, dla urządzenia, które prowadzisz sam, **Gdzie stoi urządzenie?** (zobacz [Poza obiektem](#off-the-premises)).
- **Retencja**: keep-last, dzienne, tygodniowe i miesięczne, dla każdego repozytorium w tym miejscu. Nowe miejsce zaczyna z domyślnymi regułami; miejsce ze wszystkimi regułami na zero nigdy niczego nie przycina.
- **Ochrona**: przełącznik **Append-only**. Append-only musi wymuszać druga strona; przy włączonym przełączniku BombVault nigdy niczego tam nie przycina ani nie usuwa. Na rest-serverze z włączonym append-only **Sprawdź append-only** uruchamia tamper test dla każdej ścieżki domeny, każdej włączonej kopii i każdego repozytorium w tym miejscu i pokazuje jedną odpowiedź dla całego miejsca: *usuwanie odrzucane* albo *usuwanie dozwolone* (zobacz [Kopie poza siedzibą i odzyskiwanie](offsite-recovery.md)). Tę sekcję mają tylko miejsca zdalne, bo nic na tej maszynie nie może zapobiec usunięciu lokalnego repozytorium.
- **Dostęp**: poświadczenia oraz, dla S3, klasa pamięci. Miejsce, które używa wspólnych poświadczeń, przy pierwszej zmianie dostaje własny zestaw. Repozytorium bezpośrednie w tym miejscu, którego nowe poświadczenia nie mogą otworzyć, zachowuje stare, a odpowiedź o tym informuje. Miejsca typu folder, SFTP i rclone nie mają tej sekcji.
- **Limity**: tempo wysyłania i pobierania oraz budżet wzrostu.
- **Foldery**: jeden przełącznik na domenę, z nazwą jej folderu. Domena wyłączona tutaj nie może wybrać tego miejsca.

Skrócenie przechowywania najpierw pyta o zgodę i mówi, ilu elementów to dotyczy; wyłączenie append-only najpierw pyta o zgodę i mówi, ile repozytoriów w tym miejscu straci ochronę. Wyłączenie miejsca wyłącza każde repozytorium w nim; miejsca, w którym przechowywana jest domena, nie da się wyłączyć.

## Karta Domeny {#domains}

Karta ma jeden wiersz na domenę, z jej harmonogramem, miejscem przechowywania, miejscami kopii i wyjątkami.

- **Przechowywane w**: dopóki w lokalizacji kopii zapasowych domeny nie ma żadnych kopii, wybrane miejsce staje się miejscem przechowywania domeny, a lokalizacja przenosi się tam. Gdy są już w niej kopie, wybór dla kontenerów, VM i zestawów plików staje się wartością domyślną dla nowych elementów, które przejmują ją przy pierwszej kopii; elementy, które mają już kopie, zostają tam, gdzie są, bo BombVault nigdy nie przenosi kopii zapasowej. Dla flash i własnej konfiguracji BombVault zmienia się miejsce przechowywania, a już zapisane kopie zostają w starym miejscu.
- **Kopiowane do**: jeden znacznik na każde miejsce, które może przyjmować kopie domeny. Zaznaczenie znacznika czyni miejsce celem kopii domeny; za pierwszym razem BombVault mówi z góry, ile elementów i migawek oraz ile danych wyśle pierwsze uruchomienie. Odznaczenie zatrzymuje nowe kopie: kopie, które już tam są, zostają i starzeją się według zasad przechowywania tego miejsca, a elementy z własnym wyborem nadal są tam kopiowane. Odznaczenie ostatniego znacznika zatrzymuje wszystkie kopie, także do miejsc dodanych później, dopóki znów któregoś nie zaznaczysz. Wyłączone miejsce jest pokazane jako przygaszony znacznik i nie można go wybrać.
- **Wyjątki**: elementy z własnym wyborem, jako lista z linkami do ich kart.
- **Kopiuj teraz** od razu uruchamia kopiowanie domeny.

Domena wstrzymana po odbudowie przez Odkryj pokazuje wstrzymanie w swoim wierszu, z przyciskiem **Potwierdź domyślne** (zobacz [Rozmieszczenie per element](offsite-recovery.md#placement)).

## Zmiana adresu {#addresses}

Folder domeny można zmienić w szczegółach miejsca, podobnie jak adres miejsca lokalnego, na przykład gdy repozytorium zostało ręcznie przeniesione na inny dysk. BombVault testuje każdy adres, którego dotyczy zmiana, i przyjmuje ją, gdy każdy nowy adres jest pusty, a pod starym nic nie zapisano, albo gdy każdy nowy adres zawiera to samo repozytorium restic co stary. Wszystko inne jest odrzucane, z liczbą kopii zapasowych, które wciąż leżą pod starym adresem. Miejsce zdalne zachowuje swój adres; aby tworzyć kopie zapasowe gdzie indziej, podłącz ten adres jako osobne miejsce.

BombVault buduje listę miejsc z własnej bazy danych i nigdy nie listuje zdalnego repozytorium, żeby ją wypełnić; test uruchamia się tylko wtedy, gdy coś zmieniasz.

## Usuwanie miejsca {#remove}

Miejsce można usunąć tylko wtedy, gdy nic go nie używa: żadna domena nie jest w nim przechowywana, żadna wartość domyślna na nie nie wskazuje, żaden element nie jest w nim zapisany i żadne repozytorium bezpośrednie w nim nie zawiera elementów. W przeciwnym razie odmowa wymienia, co je blokuje. Usunięcie zabiera ze sobą jego cele kopii oraz jego własne poświadczenia, chyba że używa ich źródło pobierania albo inne miejsce. W samym magazynie nic nie jest usuwane, a potwierdzenie mówi, ile kopii tam zostaje.

## Bez miejsca {#without-a-place}

Adres, który nie pasuje do postaci miejsca z folderem, działa dalej i jest wymieniony pod **Bez miejsca**, razem ze swoim adresem. Należą do nich natywne adresy `b2:`, `gs:` i `swift:`. **Przypisz do miejsca** dołącza taki wiersz do miejsca po tym samym teście co przy [zmianie adresu](#addresses). Cel kopii bez miejsca jest też wymieniony w wierszu swojej domeny, obok znaczników, i kopiuje dalej. Zdalny wiersz ma tam własny przełącznik **Append-only**, a jego wyłączenie najpierw pyta o zgodę i podaje liczbę elementów, które trzymają kopie pod tym adresem. Repozytorium bezpośrednie idzie za przełącznikiem swojego celu.

## Poza obiektem {#off-the-premises}

**Gdzie stoi urządzenie?** ma dwie odpowiedzi: **Tutaj, w tym budynku** i **W innej lokalizacji**. Kopia liczy się jako osobna lokalizacja, dla linii 3-2-1 na kartach i dla kontroli poza siedzibą na panelu, tylko wtedy, gdy jej miejsce jest w innej lokalizacji. Drugi dysk lub NAS w tym samym budynku to druga kopia, a nie druga lokalizacja. Odpowiedź nie zmienia żadnej kopii. Dostawcy chmury są zawsze w innej lokalizacji, a folder na tym serwerze Unraid zawsze tutaj, więc formularz o nie nie pyta; dla każdego innego miejsca zmień odpowiedź w jego szczegółach. Miejsce w innej lokalizacji ma w swoim wierszu oznaczenie **Inna lokalizacja**.

## Rodzaje połączeń

### Folder na tym serwerze Unraid lub na NAS {#kind-local}

Adres to ścieżka pod `/mnt`, zapisana bez `/mnt`, na przykład `user/bombvault`, a folder każdej domeny leży pod nią: `user/bombvault/container`.

- **Folder na tym serwerze Unraid** wybiera spośród udziałów, dysków i pul.
- **Synology**, **QNAP**, **TrueNAS**, **Inny Unraid** i **Inny udział** wybierają z `/mnt/remotes`. Najpierw zamontuj udział w Unraid, na przykład za pomocą wtyczki Unassigned Devices. Host Data musi być zamontowany jako Read/Write - Slave, inaczej udział, który zamontuje się po starcie BombVault, pozostaje niewidoczny aż do restartu (zobacz [Konfiguracja](configuration.md)).

W oknie wyboru folderu przycisk **Nowy folder** tworzy folder w bieżącym katalogu. Test sprawdza, czy folder jest pusty lub nie istnieje i czy BombVault może w nim zapisywać.

### S3 {#kind-s3}

Adres to `s3:https://<endpoint>/<bucket>/<path>`, na przykład `s3:https://s3.eu-central-003.backblazeb2.com/tower-backups/bombvault`.

- **Backblaze B2** potrzebuje tylko identyfikatora klucza i klucza aplikacji. BombVault pyta B2, do którego zasobnika, punktu końcowego S3 i folderu klucz jest ograniczony, i buduje z nich adres. Klucz z dostępem do wszystkich zasobników podaje je do wyboru.
- **Amazon S3**, **Cloudflare R2**, **Wasabi**, **Hetzner Object Storage**, **Storj**, **IDrive e2**, **Scaleway**, **OVHcloud**, **DigitalOcean Spaces**, **IONOS**, **Contabo**, **Exoscale** i **Vultr** proszą o klucz oraz, tam gdzie dostawca tego wymaga, o region, ID konta lub punkt końcowy. BombVault uzupełnia punkt końcowy i wyświetla zasobniki, gdy klucz może je wylistować; w przeciwnym razie wpisz nazwę zasobnika.
- **Google Cloud Storage** działa przez swój interfejs S3 z kluczem HMAC, który tworzysz w ustawieniach Cloud Storage w sekcji Interoperability. Plik konta usługi tu nie działa.
- **MinIO**, **SeaweedFS**, **Garage**, **Ceph**, **JuiceFS**, **RustFS**, **Versity S3 Gateway** i **Inna usługa S3** przyjmują adres usługi i klucz.

Klasę pamięci ustawia się w szczegółach miejsca; do wyboru są tylko warstwy, które przywracanie może odczytać bez rozmrażania.

### rest-server {#kind-rest}

Adres to `rest:<url>/<user>`, na przykład `rest:https://nas.lan:8000/tower`. Formularz pyta o adres serwera, użytkownika i hasło. Przy `--private-repos` użytkownik może sięgać tylko do ścieżek, które zaczynają się od jego własnej nazwy, więc BombVault umieszcza użytkownika na początku, chyba że wpiszesz inną ścieżkę. Gdy serwer odrzuca ścieżkę spoza własnej ścieżki użytkownika, komunikat błędu to mówi.

Formularz rest-server zawiera gotowy do wklejenia przepis na rest-server w trybie append-only z jednym użytkownikiem dla tego BombVault. **Pokaż przepis** tworzy hasło, pokazywane raz, i podaje linię `docker run`, plik compose oraz szablon Unraid, każde z linią `htpasswd` do umieszczenia na serwerze; użytkownik i hasło trafiają od razu do formularza.

**Inny BombVault** pokazuje nad swoimi polami otwarte oferty, które inne instancje wysłały przez Flotę. Przyjęcie oferty dodaje miejsce, które przechowuje kopie tylko oferowanej domeny, bo oferta zawiera użytkownika tylko dla tej jednej domeny. Przyjęcie na stronie Flota dodaje to samo miejsce.

### SFTP {#kind-sftp}

Adres to `sftp://<user>@<host>:<port>/<path>`, na przykład `sftp://bv@backup.lan:22/bombvault`. Formularz pyta o host, port i użytkownika i pokazuje klucz publiczny BombVault. Dodaj ten klucz do `~/.ssh/authorized_keys` użytkownika na serwerze; niczego innego nie trzeba tam instalować. BombVault akceptuje klucz hosta serwera przy pierwszym kontakcie i od tej pory go sprawdza.

**Hetzner Storage Box** wpisuje `<user>.your-storagebox.de` i port 23. Zainstaluj klucz na Storage Box własnym poleceniem Hetznera, które raz pyta o hasło do Storage Box:

```sh
echo '<public key>' | ssh -p 23 <user>@<user>.your-storagebox.de install-ssh-key
```

### WebDAV: Nextcloud, ownCloud, OpenCloud {#kind-webdav}

Formularz pyta o adres serwera, użytkownika i hasło aplikacji. Utwórz hasło aplikacji w ustawieniach bezpieczeństwa konta i wpisz identyfikator użytkownika, a nie adres e-mail. BombVault buduje ścieżkę WebDAV, której używa dany produkt, i przekazuje połączenie do restic przez zmienne środowiskowe rclone, z hasłem w zaciemnionej postaci rclone. Adres ma postać `rclone:bvp<id>:<path>`, gdzie `bvp<id>` to zasób zdalny, który istnieje tylko w tym środowisku; do konfiguracji rclone nic nie jest zapisywane.

### Azure Blob {#kind-azure}

Adres to `azure:<container>:/<path>`. Formularz pyta o konto magazynu i jego klucz dostępu; po **Testuj połączenie** wyświetla kontenery konta do wyboru albo wpisujesz nazwę kontenera. BombVault przekazuje konto i klucz do restic jako `AZURE_ACCOUNT_NAME` i `AZURE_ACCOUNT_KEY`.

### rclone {#kind-rclone}

Adres to `rclone:<remote>:<path>`. Formularz wyświetla do wyboru zasoby zdalne z konfiguracji rclone w BombVault. Aby zastąpić tę konfigurację, wklej cały plik `rclone.conf` pod **Konfiguracja rclone** i kliknij **Zapisz konfigurację**. Zapisuje się ona od razu i obsługuje każde miejsce rclone, niezależnie od tego, czy okno potem je doda.

## Kopie między miejscami z różnymi poświadczeniami {#different-credentials}

Domena przechowywana w miejscu zdalnym jest źródłem swoich kopii. `restic copy` działa w jednym środowisku, a BombVault dodaje poświadczenia źródła do poświadczeń celu, o ile oba nie ustawiają tej samej zmiennej na różne wartości. Miejsce Nextcloud i miejsce B2 używają różnych zmiennych, więc domenę przechowywaną w Nextcloud można kopiować do B2. Dwa konta S3 albo dwóch użytkowników rest-servera potrzebowałoby tych samych zmiennych z różnymi wartościami; restic nie przyjmie obu, a znacznik na karcie Domeny informuje, że poświadczenia do siebie nie pasują.
