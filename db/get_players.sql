DELIMITER //
DROP PROCEDURE IF EXISTS ih_login_server.get_players;
CREATE PROCEDURE ih_login_server.get_players()
BEGIN
    DECLARE account VARCHAR(64);
	DECLARE unique_id VARCHAR(64);
	DECLARE last_server_id INT;
	DECLARE s1_pid, s2_pid, s3_pid, s4_pid, s5_pid, s6_pid, s7_pid, s8_pid INT DEFAULT 0;
	DECLARE s1_name, s2_name, s3_name, s4_name, s5_name, s6_name, s7_name, s8_name VARCHAR(32) DEFAULT '';
    DECLARE done INT DEFAULT 0;

	DECLARE cur CURSOR FOR (SELECT AccountId, UniqueId, LastSelectServerId FROM ih_login_server.Account);
	DECLARE CONTINUE HANDLER FOR NOT FOUND SET done = 1;
	
	CREATE TEMPORARY TABLE ih_login_server.tmp_players (
		Account CHAR(64),
 		UniqueId CHAR(64),
		S1_PID INT(11) UNSIGNED NOT NULL,
		S1_NAME CHAR(64),
		S2_PID INT(11) UNSIGNED NOT NULL,
		S2_NAME CHAR(64),
		S3_PID INT(11) UNSIGNED NOT NULL,
		S3_NAME CHAR(64),
		S4_PID INT(11) UNSIGNED NOT NULL,
		S4_NAME CHAR(64),
		S5_PID INT(11) UNSIGNED NOT NULL,
		S5_NAME CHAR(64),
		S6_PID INT(11) UNSIGNED NOT NULL,
		S6_NAME CHAR(64),
		S7_PID INT(11) UNSIGNED NOT NULL,
		S7_NAME CHAR(64),
		S8_PID INT(11) UNSIGNED NOT NULL,
		S8_NAME CHAR(64),
 		PRIMARY KEY (Account)
 	);

	OPEN cur;
	it_loop: LOOP
		FETCH cur INTO account, unique_id, last_server_id;
		IF done = 1 THEN
			LEAVE it_loop;
		END IF;
		SELECT PlayerId, Name INTO s1_pid, s1_name FROM ih_hall_server.Players;
		SELECT PlayerId, Name INTO s2_pid, s2_name FROM ih_hall_server_2.Players;
		SELECT PlayerId, Name INTO s3_pid, s3_name FROM ih_hall_server_3.Players;
		SELECT PlayerId, Name INTO s4_pid, s4_name FROM ih_hall_server_4.Players;
		SELECT PlayerId, Name INTO s5_pid, s5_name FROM ih_hall_server_5.Players;
		SELECT PlayerId, Name INTO s6_pid, s6_name FROM ih_hall_server_6.Players;
		SELECT PlayerId, Name INTO s7_pid, s7_name FROM ih_hall_server_7.Players;
		SELECT PlayerId, Name INTO s8_pid, s8_name FROM ih_hall_server_8.Players;
		
		INSERT INTO ih_login_server.tmp_players (Account, UniqueId, S1_PID, S1_NAME, S2_PID, S2_NAME, S3_PID, S3_NAME, S4_PID, S4_NAME, S4_PID, S4_NAME, S5_PID, S5_NAME, S6_PID, S6_NAME, S7_PID, S7_NAME, S8_PID, S8_NAME)
		VALUES (account, unique_id, s1_pid, s1_name, s2_pid, s2_name, s3_pid, s3_name, s4_pid, s4_name, s5_pid, s5_name, s6_pid, s6_name, s7_pid, s7_name, s8_pid, s8_name);
	END LOOP it_loop;
	CLOSE cur;
    SELECT * INTO OUTFILE '/tmp/players.xls' FROM ih_login_server.tmp_players; 
	DROP TABLE ih_login_server.tmp_players;
END;//
DELIMITER ;
