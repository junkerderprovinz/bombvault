# Funkcje

Krótki katalog tego, co robi BombVault. Każda linia odsyła do strony, która ją objaśnia.

## Zakres kopii zapasowych {#backup-scope}

- **Containers**: folder appdata i definicja kontenera, z wyborem folderów dla każdego kontenera. [Kopie zapasowe i elementy](backups.md#what-is-saved)
- **Maszyny wirtualne**: obrazy dysków, XML i UEFI NVRAM przez SSH, z łagodnym zamknięciem, migawką na żywo albo samymi zmienionymi blokami. Maszyny z migawkami Unraid i dyskami na zvol są obsłużone w całości. [Kopie maszyn wirtualnych](vm-backup-ssh-setup.md)
- **Flash**: cały dysk USB flash Unraid, z zaplanowanym eksportem zip. [Kopie zapasowe i elementy](backups.md#settings-tab)
- **Autokopia**: własne ustawienia BombVault i dane logowania. [Kreator awaryjny](offsite-recovery.md#emergency)
- **Foldery**: dowolny folder na serwerze jako zestaw folderów. [Dodawanie folderu](backups.md#add)
- **Zbiory danych ZFS**: drzewo zbiorów danych odczytane z jednej migawki, z możliwością przywrócenia pojedynczych plików i opcjonalną repliką na drugim serwerze ZFS. [Zbiory danych ZFS](zfs-datasets.md)
- **Zrzuty bazy danych**: PostgreSQL, MySQL i MariaDB, zrzucane z działającego serwera do osobnego punktu przywracania. [Zrzuty baz danych](backups.md#database-dumps)

## Kopie zapasowe i strony elementów {#backups}

- **Jedna lista** z kafelkami, **Filtry**, **Sortuj** i **Szukaj**, z czternastoma ostatnimi przebiegami każdego wiersza jako paskiem. [Lista](backups.md#list)
- **Zaznaczanie jak w menedżerze plików** oraz kopia lub wstrzymanie zaznaczenia na serwerze. [Wybieranie kilku elementów](backups.md#select)
- **Strona dla każdego elementu** z kartami **Kopie zapasowe**, **Ustawienia** i **Weryfikacja**. [Strona elementu](backups.md#entry-page)
- **Tagi i porównywanie** migawek oraz **Eksport** jako tar, zip albo repozytorium restic. [Karta Kopie zapasowe](backups.md#backups-tab)
- **Wzorce wykluczeń** z podglądem na żywo i polecane wykluczenia dla znanych aplikacji. [Wzorce wykluczeń](backups.md#excludes)
- **Polecenia przed i po** oraz **Zatrzymaj inne kontenery** z restartem w kolejności zależności, bramkowanym stanem zdrowia. [Polecenia i inne kontenery](backups.md#commands)
- **Gdy aplikacja jest zajęta**: zaplanowana kopia czeka na Plex, Jellyfin, Emby, Twoją własną kontrolę albo spokojny ruch i sama próbuje ponownie. [Gdy aplikacja jest zajęta](backups.md#app-busy)
- **Migawka zamiast zatrzymania**: kontener na ZFS jest wyłączony tylko na kilka sekund. [Harmonogramy](schedules.md#snapshot-instead-of-stop)
- **Zwykłe eksporty**, opcjonalnie zabezpieczone przez age. [Zwykłe eksporty](backups.md#exports)

## Przywracanie {#restore}

- **Jedna strona i jedno okno** dla każdego rodzaju, z zaznaczoną z góry zalecaną kopią. [Przywracanie](restore.md)
- **Obok oryginału** domyślnie, a **Zastąp oryginał** najpierw kopiuje bieżący stan. [Okno przywracania](restore.md#window)
- **Pojedyncze pliki** do folderu albo z powrotem na ich miejsce. [Odzyskaj jeden plik](restore.md#example-single-file)
- **Kilka elementów naraz**, każdy z zalecanej kopii albo z jednego momentu. [Przywracanie kilku elementów](restore.md#several)
- **Całe stacki Compose**, uruchamiane w kolejności zależności. [Przywracanie stacku](restore.md#stacks)
- **Kontenery i maszyny wirtualne są tworzone na nowo** na obrazie, na którym działały, z GPU, limitami i linkami. [Co wraca](restore.md#kinds)
- **Kontrole przed przywracaniem** i lista tego, co zmienia. [Okno przywracania](restore.md#window)
- **Flash jako ZIP** i po jednej wtyczce z powrotem na działający flash. [Co wraca](restore.md#kinds)
- **ZFS** do zbioru danych z migawką bezpieczeństwa, do folderu albo do nowego zbioru danych z jego właściwościami. [Zbiory danych ZFS](zfs-datasets.md#restore)
- **Kreator awaryjny** dla utraconego serwera, z zestawem odzyskiwania, przez grupę albo z wpisanym kluczem. [Kreator awaryjny](offsite-recovery.md#emergency)
- **Ze starego folderu z kopiami**, **Import z wtyczki Appdata.Backup** i **Zadania z Duplicati lub Duplicacy**, gdzie ich zadania kopii zapasowych stają się zestawami folderów. [Inne źródła](restore.md#other-sources)

## Pamięć i harmonogram {#storage-scheduling}

- **Miejsca docelowe** każdego rodzaju: foldery lokalne, SMB, WebDAV, SFTP, rest-server, S3, dyski w chmurze i dowolny zasób rclone. [Miejsca docelowe](offsite-recovery.md#destinations)
- **Dane logowania**: jedna lista z ustawieniem domyślnym. [Dane logowania](offsite-recovery.md#sign-ins)
- **Własne foldery** na tym serwerze albo w miejscu docelowym. [Własny folder sekcji](offsite-recovery.md#own-folder)
- **3-2-1 jako przepływ**, w którym każda linia jest przełącznikiem, oraz **ustawienie dla elementu**. [Ustawienie dla elementu](offsite-recovery.md#placement)
- **Kopie poza siedzibą** do kilku miejsc na sekcję, z limitami przepustowości, kompresją, klasami zimnej pamięci i zwolnieniem, gdy Plex streamuje. [Kopie poza siedzibą](offsite-recovery.md#off-site-copies)
- **Repozytoria bezpośrednie**, wymienione pod **Kopie trafiają prosto tutaj**. [Repozytoria bezpośrednie](offsite-recovery.md#direct-repositories)
- **Przechowywanie jako oś czasu**: **Krótko**, **Zrównoważone**, **Długo** albo **Własne wartości** dla każdego miejsca, z regułami dla sekcji. [Jak długo przechowywać](offsite-recovery.md#keep)
- **Przenoszenie historii sekcji** i **Usuń miejsce docelowe** z kopiami lub bez nich. [Przenoszenie i usuwanie](offsite-recovery.md#move-remove)
- **Harmonogramy czytane jak zdania**, pasek **Tydzień**, pełna kopia, plany według sekcji i własny plan dla elementu. [Harmonogramy](schedules.md)
- **Nadrabianie pominiętych kopii**, ręczna kolejność i aktualizacje po udanej kopii. [Harmonogramy](schedules.md)

## Wgląd, weryfikacja i monitorowanie {#insight-verification-monitoring}

- **Przegląd**: nagłówek stanu, **Wymaga uwagi**, **Stan ochrony**, prognoza pamięci, historia i opcjonalne karty. [Przegląd](overview.md)
- **Wykrywanie anomalii** dla każdego elementu, które zachowuje też stare kopie, gdy źródło się załamuje. [Anomalie](anomalies.md)
- **Sprawdzenie przywracania** po pierwszej kopii i **Test uruchomienia** w odizolowanej kopii. [Karta Weryfikacja](backups.md#verify-tab)
- **Zaplanowane kontrole i próbne przywracanie** z kopii poza siedzibą. [Próby DR](offsite-recovery.md#dr-drills)
- **Rozmiar według folderu** i dlaczego kopia była wolna. [Co mówi o sobie przebieg](backups.md#run-notes)
- **Powiadomienia**: webhook, Gotify, Matrix, e-mail, Apprise, Unraid i Healthchecks, z cotygodniowym podsumowaniem i raportem o opóźnieniach. [Powiadomienia](notifications.md)
- **Prometheus**, urządzenie Home Assistant, API HTTP i mDNS. [API i integracje](api.md)
- **Zgłoszenie błędu**: ZIP z usuniętymi danymi wrażliwymi do zgłoszenia na GitHubie. [Rozwiązywanie problemów](troubleshooting.md#still-stuck)

## Ochrona przed ransomware {#ransomware-protection}

- **Ochrona przed usunięciem (append-only)**, wymuszana po drugiej stronie i udowadniana testem manipulacji. [Ochrona przed usunięciem](offsite-recovery.md#delete-protection-append-only)
- **Akceptuj bez ochrony przed usunięciem** dla miejsca, które nie potrafi odmówić usunięcia. [Ochrona przed usunięciem](offsite-recovery.md#delete-protection-append-only)
- **Trwałe usuwanie kopii wymaga twojego własnego logowania**, gdy ustawione jest hasło logowania, a sparowany telefon nie może tego zrobić. [Model bezpieczeństwa](configuration.md#security-model)
- **Zestaw odzyskiwania**, opcjonalnie zabezpieczony przez age, oraz **Przechowawca klucza** w Twojej grupie. [Zestaw odzyskiwania](offsite-recovery.md#recovery-kit), [Przechowawca klucza](offsite-recovery.md#key-keeper)

## Kilka serwerów {#several-servers}

- **Parowanie dwunastoma słowami**, szyfrowane od końca do końca, bezpośrednio albo przez relay. [Parowanie instancji](offsite-recovery.md#pairing)
- **Strona Instancje** z grupą jako siecią i rolami **Odbiornik**, **Pobierający**, **Serwer ZFS** i **Przechowawca klucza**. [Instancje i role](offsite-recovery.md#roles)
- **Serwer odbiorczy** ustawiony za Ciebie, w trybie append-only. [Serwer odbiorczy](offsite-recovery.md#receiving-server)
- **Wspólne repozytorium**, do którego kopiuje kilka serwerów, więc to, co mają wspólne, jest zapisane tylko raz. Przycina je tylko serwer, który jest jego właścicielem. [Wspólne repozytorium](offsite-recovery.md#shared-repository)

## Asystenci AI (MCP) {#mcp}

Wbudowany serwer MCP pozwala asystentowi czytać stan kopii, pokrycie, historię przebiegów i punkty przywracania, a z kluczem, który na to pozwala, uruchomić kopię. Przywracanie, usuwanie, przycinanie i ustawienia zostają w interfejsie webowym. [Serwer MCP](mcp.md)

## Aplikacje i towarzysze {#apps}

- **Aplikacja na Androida**: każdy serwer Twojej grupy w telefonie. [Aplikacja na Androida](android.md)
- **BombVault Widget**: kafelek na Pulpicie Unraid. [Aplikacje](settings.md#apps)
- **Osadzany dziennik aktywności** dla pulpitów takich jak Homepage czy Heimdall. [Widżet pulpitu](api.md#dashboard-widget)

## Inne {#other}

- **42 języki**, trzy motywy oraz etykiety, kolory, narożniki i animacja według Twojego wyboru. [Wygląd](settings.md#look)
- **Ustawienia, które zapisują się same**, z wyszukiwaniem we wszystkich czternastu kafelkach. [Ustawienia](settings.md)
- **Nowości** po aktualizacji, z zielonymi kropkami przy tym, co się zmieniło. [Info](settings.md#info)
- **HTTPS od razu po instalacji** i healthcheck dla narzędzi samonaprawiających. [Konfiguracja](configuration.md#network)
