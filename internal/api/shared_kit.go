package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/platform"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/template"
)

// kitSharedSection is the heading of the recovery kit's chapter on shared
// repositories.
const kitSharedSection = "Shared repositories"

// kitSharedFieldRe is a line under a shared repository of the kit that a new
// server reads back.
var kitSharedFieldRe = regexp.MustCompile(`^  (owner|sections|other|server port|server folder|server login): (.+)$`)

// kitOtherRe is another server of a shared repository as the kit names it.
var kitOtherRe = regexp.MustCompile(`^(.+), host (\S+)(?:, group id (\S+))?(?:, login (\S+))?$`)

// kitSharedServer is the rest-server the server that held a shared repository
// ran for the others: its port, its data folder below the host data mount,
// where the logins of the others are kept, and the login of its own.
type kitSharedServer struct {
	Port   int
	Folder string
	User   string
}

// sharedKitSection writes the kit's chapter on the shared repositories this
// server copies into or holds. Each names the host this server writes there,
// which every restic command has to select by, and the other servers whose
// backups the same password opens.
func (s *Service) sharedKitSection(w func(string, ...any), settings store.Settings, key string) {
	repos, err := s.store.ListSharedRepos()
	if err != nil {
		log.Printf("api: recovery kit: the shared repositories could not be read: %v", err)
		return
	}
	if len(repos) == 0 {
		return
	}
	targets, err := s.store.ListOffsiteTargets()
	if err != nil {
		log.Printf("api: recovery kit: the copy targets could not be read: %v", err)
	}
	joins, err := s.store.ListSharedJoins(store.SharedJoinOut)
	if err != nil {
		log.Printf("api: recovery kit: the shared repositories this server joined could not be read: %v", err)
	}
	logins := map[string]string{}
	if in, err := s.store.ListSharedJoins(store.SharedJoinIn); err != nil {
		log.Printf("api: recovery kit: the members of the shared repository this server holds could not be read: %v", err)
	} else {
		for _, j := range in {
			if j.State == store.SharedJoinJoined && j.User != "" {
				logins[j.MemberID] = j.User
			}
		}
	}
	host := restickey.Host(key)
	w("## " + kitSharedSection + "\n\n")
	w("These restic repositories are shared with other BombVault servers of the group.\n")
	w("Every server writes its backups there under a restic host of its own, this one\n")
	w("under %s, so name that host in every restic command, as shown below each\n", host)
	w("repository. The password above opens the backups of the other servers there too,\n")
	w("because every key of a restic repository opens the same master key. Their\n")
	w("containers and VMs come back as files only, since each server seals its own\n")
	w("definitions.\n\n")
	for _, sr := range repos {
		owner := s.ownsSharedRepo(sr)
		name, loc := s.sharedKitName(sr, owner, settings, joins), sr.Repo
		if owner {
			if resolved, err := s.resolveRepo(sr.Repo); err == nil {
				loc = resolved
			}
		}
		w("- %s: %s\n", name, loc)
		w("  owner: %s\n", sr.Owner)
		var sections []string
		for _, domain := range offsiteConfigDomains {
			if slices.ContainsFunc(targets, func(t store.OffsiteTarget) bool { return t.Domain == domain && sameRepoLocation(t.Repo, sr.Repo) }) {
				sections = append(sections, domain)
			}
		}
		if len(sections) > 0 {
			w("  sections: %s\n", strings.Join(sections, ", "))
		}
		for _, m := range sr.Members {
			if m.Host == host {
				continue
			}
			// The server that takes the kit over needs the login to take a
			// member out again.
			if login := logins[m.Instance]; owner && login != "" {
				w("  other: %s, host %s, group id %s, login %s\n", m.Name, m.Host, m.Instance, login)
			} else {
				w("  other: %s, host %s, group id %s\n", m.Name, m.Host, m.Instance)
			}
		}
		if owner {
			w("  This server holds it and is the only one that forgets and prunes there.\n")
			if ss, ok, err := s.store.GetSharedRepoServer(); err == nil && ok && ss.Repo == sr.Repo {
				w("  server port: %d\n", ss.Port)
				w("  server folder: %s\n", ss.Folder)
				w("  server login: %s\n", ss.User)
			}
		}
		w("    restic -r %s snapshots --host %s\n\n", loc, host)
	}
}

// sharedKitName is what the kit calls a shared repository: by the server that
// holds it.
func (s *Service) sharedKitName(sr store.SharedRepo, owner bool, settings store.Settings, joins []store.SharedJoin) string {
	if owner {
		return instanceDisplayName(settings)
	}
	if i := slices.IndexFunc(joins, func(j store.SharedJoin) bool { return j.MemberID == sr.Owner && j.MemberName != "" }); i >= 0 {
		return joins[i].MemberName
	}
	if i := slices.IndexFunc(sr.Members, func(m store.SharedMember) bool { return m.Instance == sr.Owner && m.Name != "" }); i >= 0 {
		return sr.Members[i].Name
	}
	return "Shared repository"
}

// readSharedKitField takes one line under a shared repository of the kit.
func (loc *kitLocation) readSharedKitField(field, value string) {
	switch field {
	case "owner":
		loc.Owner = value
	case "sections":
		for d := range strings.SplitSeq(value, ", ") {
			if slices.Contains(offsiteConfigDomains, d) && !slices.Contains(loc.Sections, d) {
				loc.Sections = append(loc.Sections, d)
			}
		}
	case "other":
		if m := kitOtherRe.FindStringSubmatch(value); m != nil {
			loc.Others = append(loc.Others, store.SharedMember{Name: m[1], Host: m[2], Instance: m[3]})
			if m[4] != "" {
				if loc.Logins == nil {
					loc.Logins = map[string]string{}
				}
				loc.Logins[m[2]] = m[4]
			}
		}
	case "server port":
		if port, err := strconv.Atoi(value); err == nil && port > 0 && port < 65536 {
			loc.Server.Port, loc.Held = port, true
		}
	case "server folder":
		loc.Server.Folder = value
	case "server login":
		loc.Server.User = value
	}
}

// attachSharedPlace records a shared repository a recovery kit names, so this
// server, now writing under the host of the one that wrote the kit, lists and
// restores only what that one wrote there. A kit of the server that held the
// repository makes this one its holder in that server's place, with the
// rest-server the others reach it through. Each section that copied there gets
// its target again. It says which record it made, empty when there was one,
// and which targets.
func (s *Service) attachSharedPlace(ctx context.Context, p emergencyPlace, ref string) (string, []string, error) {
	// The holder's own server and copies name the repository by its folder
	// below the host data mount.
	repo := p.Location
	if p.Held && p.Folder != "" {
		repo = p.Folder
	}
	recorded := ""
	_, known, err := s.store.SharedRepoAt(repo)
	if err != nil {
		return "", nil, err
	}
	if !known {
		g, err := s.store.GetGroupState()
		if err != nil {
			return "", nil, err
		}
		settings, err := s.store.GetSettings()
		if err != nil {
			return "", nil, err
		}
		here := s.resticHost()
		members := slices.DeleteFunc(slices.Clone(p.Others), func(m store.SharedMember) bool { return m.Host == here })
		members = append(members, store.SharedMember{Host: here, Instance: g.InstanceID, Name: instanceDisplayName(settings)})
		owner := p.Owner
		if p.Held {
			owner = g.InstanceID
		}
		sr, err := s.store.PutSharedRepo(store.SharedRepo{Repo: repo, Owner: owner, Members: members})
		if err != nil {
			return "", nil, err
		}
		recorded = sr.ID
		if p.Held {
			if err := s.keepSharedJoins(members, p.Logins); err != nil {
				return recorded, nil, err
			}
			if err := s.store.QueueSharedHolderNotices(sr.ID, sharedHolderAudience(p.Owner, members, g.InstanceID)); err != nil {
				return recorded, nil, err
			}
			if err := s.takeBackSharedServer(ctx, repo, p.Server); err != nil {
				log.Printf("api: emergency: the server of the shared repository did not come back, so set the repository up again on the Instances page: %v", err)
			}
		}
	}
	if !p.Held && !restic.IsRemoteRepo(repo) {
		return recorded, nil, nil
	}
	name := "Shared with " + p.Name
	if p.Held {
		name = "Shared repository"
	}
	var made []string
	for _, domain := range p.Sections {
		at := p
		at.Domain, at.Name, at.Location = domain, name, repo
		id, err := s.attachOffsiteTarget(at, ref)
		if err != nil {
			return recorded, made, err
		}
		if id != "" {
			made = append(made, id)
		}
	}
	return recorded, made, nil
}

// keepSharedJoins records the members the kit names a login for as joined,
// so that this server can take them out again as their holder.
func (s *Service) keepSharedJoins(members []store.SharedMember, logins map[string]string) error {
	s.sharedMu.Lock()
	defer s.sharedMu.Unlock()
	for _, m := range members {
		user := logins[m.Host]
		if user == "" || m.Instance == "" {
			continue
		}
		j := store.SharedJoin{Direction: store.SharedJoinIn, MemberID: m.Instance, MemberName: m.Name, Host: m.Host, State: store.SharedJoinJoined, User: user, DecidedAt: time.Now().Unix()}
		if err := s.store.SaveSharedJoin(j); err != nil {
			return err
		}
	}
	return nil
}

// takeBackSharedServer brings back the rest-server the other servers reach a
// shared repository through, on the data folder that kept their logins. A
// container that still runs under its name stays as it is.
func (s *Service) takeBackSharedServer(ctx context.Context, repo string, k kitSharedServer) error {
	if _, ok, err := s.store.GetSharedRepoServer(); err != nil || ok {
		return err
	}
	if k.User == "" {
		return errors.New("the recovery kit names no login of the server")
	}
	in, hostPath, err := s.checkReceiverInput(receiverServerInput{Folder: k.Folder, Port: k.Port})
	if err != nil {
		return err
	}
	ss := store.SharedRepoServer{ContainerName: sharedServerName, Folder: in.Folder, HostPath: hostPath, Port: in.Port, User: k.User, Repo: repo}
	if _, ss.PasswordEnc, err = s.addHtpasswdUser(ss.Folder, ss.User); err != nil {
		return err
	}
	live, err := s.docker.InspectName(ctx, ss.ContainerName)
	if err != nil {
		return err
	}
	if live != "" {
		s.reloadContainer(ctx, ss.ContainerName)
	} else if err := s.startSharedServer(ctx, ss); err != nil {
		return err
	}
	return s.store.SaveSharedRepoServer(ss)
}

// startSharedServer creates and starts the shared repository's rest-server.
func (s *Service) startSharedServer(ctx context.Context, ss store.SharedRepoServer) error {
	if err := s.refuseSecondSharedServer(ctx, ss.Port); err != nil {
		return err
	}
	if id, err := s.docker.ImageID(ctx, restServerImage); err != nil {
		return err
	} else if id == "" {
		if err := s.docker.Pull(ctx, restServerImage); err != nil {
			return fmt.Errorf("download %s: %w", restServerImage, err)
		}
	}
	unraid := s.platformFn().Kind() == platform.KindUnraid
	if err := s.docker.CreateAndStart(ctx, sharedContainerSpec(ss.HostPath, ss.Port, unraid), true); err != nil {
		return fmt.Errorf("create the %s container: %w", sharedServerName, err)
	}
	if unraid {
		if err := template.Write(s.cfg.FlashTemplatesDir, ss.ContainerName, sharedServerTemplate(ss)); err != nil {
			log.Printf("shared repository: write the Unraid template: %v", err)
		}
	}
	return nil
}

// sharedHolderAudience is who has to hear that the server self holds a shared
// repository now: the server that held it until then and every member.
func sharedHolderAudience(previous string, members []store.SharedMember, self string) []string {
	var out []string
	add := func(in string) {
		if in != "" && in != self && !slices.Contains(out, in) {
			out = append(out, in)
		}
	}
	add(previous)
	for _, m := range members {
		add(m.Instance)
	}
	return out
}
