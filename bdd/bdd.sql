CREATE DATABASE IF NOT EXISTS bdd_messagerie;

USE bdd_messagerie;


-- Comptes --

CREATE TABLE IF NOT EXISTS compte (
idCompte INT AUTO_INCREMENT PRIMARY KEY,
identifiant VARCHAR(15) UNIQUE NOT NULL,
motdepasse VARBINARY(255) NOT NULL,
iv VARBINARY(16) NOT NULL,
cle VARBINARY(32) NOT NULL
);

-- Comptes de test --
INSERT INTO compte (identifiant, motdepasse, iv, cle)
VALUES
('user1', 0x1234567890123456, 0x1234567890123456, 0x123456789012345678901234567890123456),
('user2', 0x1234567890123456, 0x1234567890123456, 0x123456789012345678901234567890123456);


-- Contacts de chaque utilisateur --

CREATE TABLE IF NOT EXISTS contact (
    idContact INT AUTO_INCREMENT PRIMARY KEY,

    idPossesseur INT NOT NULL,
    idDestinataire INT NOT NULL,

    statut ENUM('en_attente', 'accepte', 'bloque') NOT NULL DEFAULT 'en_attente',

    date_creation DATETIME DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_possesseur
        FOREIGN KEY (idPossesseur) REFERENCES compte(idCompte)
        ON DELETE CASCADE,

    CONSTRAINT fk_destinataire
        FOREIGN KEY (idDestinataire) REFERENCES compte(idCompte)
        ON DELETE CASCADE,

    CONSTRAINT uc_contact UNIQUE (idPossesseur, idDestinataire)
);


-- Messages --

CREATE TABLE IF NOT EXISTS messages (
idMessage INT AUTO_INCREMENT PRIMARY KEY,
idEmetteur INT NOT NULL,
idReceveur INT NOT NULL,

contenu TEXT NOT NULL,
chemin VARCHAR(255) DEFAULT NULL,

date_creation DATETIME DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_emetteur
        FOREIGN KEY (idEmetteur) REFERENCES compte(idCompte)
        ON DELETE CASCADE,

    CONSTRAINT fk_receveur
        FOREIGN KEY (idReceveur) REFERENCES compte(idCompte)
        ON DELETE CASCADE
);

-- Ajout de contact test --
INSERT INTO contact (idPossesseur, idDestinataire, statut)
VALUES (2, 1, 'accepte');
INSERT INTO contact (idPossesseur, idDestinataire, statut)
VALUES (1, 2, 'accepte');

-- Messages de test --
INSERT INTO messages (idEmetteur, idReceveur, contenu, date_creation) VALUES
(1, 2, 'Salut ! Ça va ?', '2026-01-29 10:02:00'),
(2, 1, 'Oui nickel, et toi ?', '2026-01-29 10:02:30'),
(1, 2, 'Tranquille, je teste la messagerie que je suis en train de coder 😄', '2026-01-29 10:03:10'),
(2, 1, 'Ah stylé ! Ça fonctionne bien ?', '2026-01-29 10:03:40'),
(1, 2, 'Oui, il me manquait juste des messages pour voir le design', '2026-01-29 10:04:15'),
(2, 1, 'Classique 😅 Toujours le problème du contenu de test', '2026-01-29 10:04:50'),
(1, 2, 'Exactement. Merci de servir de cobaye 😂', '2026-01-29 10:05:20'),
(2, 1, 'Avec plaisir, tant que je suis pas payé en bugs 😜', '2026-01-29 10:05:55');





