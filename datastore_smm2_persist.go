package nex

// Persistance des profils SMM2.
//
// POURQUOI. Les profils vivaient en memoire : chaque redemarrage du serveur les
// effacait, et le joueur devait refaire son Mii, son nom et son pays. Pendant une
// soiree de mise au point ou l'on redeploie vingt fois, c'est vingt reinscriptions.
//
// COMMENT, ET POURQUOI PAS AUTREMENT. Un seul fichier JSON, reecrit UNIQUEMENT quand un
// profil change — c'est-a-dire a l'inscription, pas a chaque lecture. Ce detail n'est
// pas cosmetique : accounts.json a fait tomber le serveur le 12 aout parce qu'il etait
// reserialise en entier a CHAQUE appel, trente fois par minute, avec les avatars
// dedans. Ici les profils sont minuscules (un Mii fait 88 octets) et n'evoluent qu'a
// l'inscription, donc un fichier unique est le bon choix — mais la regle qui compte
// reste la meme : n'ecrire que sur changement.
//
// Ecriture ATOMIQUE : fichier temporaire puis rename. Une coupure au mauvais moment
// laisserait sinon un JSON tronque, et au redemarrage suivant on perdrait TOUS les
// profils au lieu d'un seul.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var (
	smm2ProfilsFichier = os.Getenv("SMM2_PROFILES_FILE")
	smm2EcritureMu     sync.Mutex
)

func fichierProfils() string {
	if smm2ProfilsFichier != "" {
		return smm2ProfilsFichier
	}
	dir := os.Getenv("STORAGE_DIR")
	if dir == "" {
		dir = "."
	}
	return filepath.Join(dir, "smm2_profils.json")
}

// SMM2ChargerProfils relit les profils depuis le disque. A appeler au demarrage.
// Un fichier absent n'est PAS une erreur : c'est un serveur neuf.
func SMM2ChargerProfils() {
	chemin := fichierProfils()
	raw, err := os.ReadFile(chemin)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("[DataStoreSMM2] profils illisibles (%v) — on repart a vide\n", err)
		}
		return
	}
	var m map[string]SMM2Profil
	if err := json.Unmarshal(raw, &m); err != nil {
		// On NE repart PAS a vide en silence : si le fichier existe mais ne se lit
		// pas, c'est un incident qui merite d'etre vu, pas une remise a zero discrete.
		fmt.Printf("[DataStoreSMM2] profils corrompus (%v) — fichier conserve, rien charge\n", err)
		return
	}
	n := 0
	for _, p := range m {
		if p.PID != 0 {
			smm2Profils.Store(p.PID, p)
			n++
		}
	}
	fmt.Printf("[DataStoreSMM2] %d profil(s) charge(s) depuis %s\n", n, chemin)
}

// smm2SauverProfils ecrit tous les profils. Appele apres chaque inscription.
func smm2SauverProfils() {
	chemin := fichierProfils()
	smm2EcritureMu.Lock()
	defer smm2EcritureMu.Unlock()

	m := map[string]SMM2Profil{}
	smm2Profils.Range(func(k, v any) bool {
		p := v.(SMM2Profil)
		m[fmt.Sprintf("%d", p.PID)] = p
		return true
	})
	raw, err := json.Marshal(m)
	if err != nil {
		fmt.Printf("[DataStoreSMM2] serialisation des profils impossible : %v\n", err)
		return
	}

	tmp := chemin + ".tmp"
	if err := os.MkdirAll(filepath.Dir(chemin), 0o755); err != nil {
		fmt.Printf("[DataStoreSMM2] dossier des profils inaccessible : %v\n", err)
		return
	}
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		fmt.Printf("[DataStoreSMM2] ecriture des profils impossible : %v\n", err)
		return
	}
	if err := os.Rename(tmp, chemin); err != nil {
		fmt.Printf("[DataStoreSMM2] remplacement du fichier de profils impossible : %v\n", err)
		_ = os.Remove(tmp)
	}
}
