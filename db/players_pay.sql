DELIMITER //
USE ih_admin //
DROP PROCEDURE IF EXISTS get_players_pay//
CREATE PROCEDURE get_players_pay()
	SELECT Account, PlayerId, PayTimeStr, BundleId INTO OUTFILE '/tmp/apple_pays.xls' FROM ApplePays GROUP BY Account ORDER BY PayTime DESC;
	SELECT Account, PlayerId, PayTimeStr, BundleId INTO OUTFILE '/tmp/google_pays.xls' FROM GooglePays GROUP BY Account ORDER BY PayTime DESC;
BEGIN
END;//
DELIMITER ;

CALL get_players_pay();