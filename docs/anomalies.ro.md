# Anomalii

Un ransomware și o aplicație stricată arată la fel într-un backup: brusc cea mai mare parte a datelor este nouă sau a dispărut. BombVault compară fiecare backup cu istoricul propriu al elementului respectiv și ridică o constatare când ceva pare în neregulă. Cât timp o constatare despre date pierdute este deschisă, păstrează și backupurile vechi ale elementului, ca ultimul backup bun să nu poată expira. Intrarea **Anomalii** din bara laterală apare cât timp detectarea este pornită.

![Pagina Anomalii cu o constatare critică deschisă.](assets/screenshots/anomalies/page-dark.png#only-dark)
![Pagina Anomalii cu o constatare critică deschisă.](assets/screenshots/anomalies/page-light.png#only-light)

## Pagina {#page}

Plăcile **Critică**, **Avertismente** și **Observații** aleg o gravitate, iar **Deschisă** și **Închise** de deasupra listei comută între ce este deschis și ce s-a închis în ultimele 30 de zile. O constatare critică stă pe aceeași suprafață ca celelalte, iar gravitatea ei se vede în insignă și în marginea colorată. O constatare îți oferă mai întâi remedierea, iar detaliile ei arată valoarea ca o curbă peste intervalul obișnuit.

Când nu este nimic deschis, **Progresul învățării** arată un inel din zece părți pentru fiecare element, câte o parte pentru fiecare backup din care a învățat BombVault. **Monitorizare** deschide lista elementelor cu setările lor proprii.

## Ce verifică {#checks}

Fiecare backup al unui container, al unei VM, al unui set de foldere, al unui dump de bază de date, al unității flash și al backupului propriu este comparat cu istoricul elementului. Verificările urmăresc:

- datele noi stocate de o rulare, față de cele mai mari cantități obișnuite din backupurile recente și față de ritmul obișnuit pe oră,
- un backup care a stocat din nou cea mai mare parte a datelor, inclusiv fișierele redenumite și rescrise,
- dimensiunea sursei și numărul de fișiere raportate de restic pentru fiecare element și fiecare dump de bază de date,
- timpul propriu de backup al lui restic,
- seriile de eșecuri și eșecurile intermitente,
- verificările de restaurare care nu mai trec,
- spațiul liber al depozitelor locale, SFTP și rclone, estimat din ritmul de creștere al depozitului.

Un element învață din primele 10 backupuri. O sursă aproape goală, o rescriere a majorității datelor și eșecurile sunt verificate încă de la început.

O constatare despre dimensiunea sau numărul de fișiere al unei surse este datată la primul backup în care s-a văzut. **Compară cu copia anterioară** listează folderele în care fișiere au dispărut, au apărut sau s-au schimbat, cu o notă când aproape tot se află într-un index de căutare, într-un cache sau în miniaturi pe care aplicația le reconstruiește singură.

Elementele ZFS sunt urmărite set de date cu set de date, vezi [Seturi de date ZFS](zfs-datasets.md#anomalies). Un asistent poate citi constatările deschise prin [serverul MCP](mcp.md#tools).

Verificările au limite. Depozitele S3, B2 și REST nu au o cifră pentru spațiul liber, pe share-ul de utilizator Unraid spațiul liber este cel al întregului array, iar backupurile făcute înainte de restic 0.17 nu au istoric de dimensiuni.

## Închiderea unei constatări {#closing}

- **Avertismentele** se închid singure după ce cauza a dispărut.
- Constatările **critice** despre date pierdute și un disc care se umple rămân până acționezi.
- **Confirmă** marchează o constatare ca văzută și o închide. Nu mai este ridicată până când cauza ei nu a dispărut o dată.
- **Marchează ca așteptată** spune că schimbarea a fost a ta. Noul nivel devine normal după 10 backupuri, dar verificarea sursei aproape goale nu este oprită niciodată.
- O selecție schimbată, de exemplu un folder bifat sau debifat, începe singură istoricul elementului de la zero.

O notificare pleacă o singură dată pe episod, iar eșecurile și verificările de restaurare care notifică deja nu sunt raportate de două ori.

## Backupurile vechi sunt păstrate între timp {#retention-hold}

Cât timp o sursă este aproape goală, s-a micșorat brusc sau cea mai mare parte a datelor ei a fost stocată din nou, retenția și curățarea (prune) nu ating backupurile vechi ale acelui element, iar constatarea trimite la ultimul backup bun, pe care fereastra de restaurare îl preselectează. Înainte să închizi o astfel de constatare, BombVault te avertizează că următoarea curățare va șterge din nou backupurile vechi după regulile de păstrare. Așa că, dacă lipsesc date sau au fost criptate, restaurează mai întâi și închide constatarea după aceea.

## Setări {#settings}

Cardul **Anomalii** de la **Setări, Verificare** conține setările globale. Fiecare control se salvează imediat ce îl schimbi, iar cele trei de sub comutator sunt ascunse cât timp detectarea este oprită.

| Setare | Implicit | Ce face |
|---|---|---|
| **Detectează anomalii** | Pornit | Compară fiecare backup cu istoricul propriu al elementului. Dacă este oprită, nu se mai verifică nimic nou, iar intrarea **Anomalii** dispare din bara laterală. Cardul trimite în continuare la constatările anterioare. |
| **Sensibilitate** | Echilibrată | Strictă raportează schimbări mai mici, Permisivă doar pe cele mari. |
| **Trimite o notificare pentru** | Doar constatări critice | Cea mai mică gravitate care trimite un mesaj prin canalele configurate la [Notificări](notifications.md). Eșecurile repetate de backup și de dump și verificările de restaurare programate eșuate trimit deja propriul mesaj și nu sunt trimise de două ori. |
| **Păstrează copiile vechi când o sursă se micșorează brusc sau este rescrisă** | Pornit | Cât timp un element are o constatare deschisă pentru o sursă aproape goală, o micșorare bruscă sau cea mai mare parte a datelor stocată din nou, retenția și curățarea (prune) nu ating backupurile vechi ale elementului. Confirmă constatarea sau marchează-o ca așteptată ca să le lași să plece. |

Fiecare element poate folosi propria sensibilitate și propriul minim de notificare. Setează-le în fila **Verificare** a elementului sau în pagina **Anomalii**, la **Monitorizare**. Pentru un element ZFS se aplică tuturor seturilor de date din arborele lui.
