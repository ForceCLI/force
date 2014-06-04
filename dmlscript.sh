#!/bin/bash
echo force record create Merchandise__c Name:"Bivalve Humus Pump" Price__c:"5.00" Quantity__c:10 Condition__c:"New" Warehouse__c:a04i000000C8pTP
read XNAME
force record create Merchandise__c Name:"Bivalve Humus Pump" Price__c:"5.00" Quantity__c:10 Condition__c:"New" Warehouse__c:a04i000000C8pTP
read XNAME
echo force query "Select Id, Name, Quantity__c, Price__c, Condition__c From Merchandise__c Where Condition__c = 'New'"
read XNAME
force query "Select Id, Name, Quantity__c, Price__c, Condition__c From Merchandise__c Where Condition__c = 'New'"
read XNAME
echo force record update Merchandise__c $XID Condition__c:Used
read XID
force record update Merchandise__c $XID Condition__c:Used
read XNAME
echo force query "Select Id, Name, Quantity__c, Price__c, Condition__c From Merchandise__c Where Condition__c = 'Used'"
read XNAME
force query "Select Id, Name, Quantity__c, Price__c, Condition__c From Merchandise__c Where Condition__c = 'Used'"
read XNAME
echo force record delete Merchandise__c $XID
read XNAME
force record delete Merchandise__c $XID
read XNAME
echo force query "Select Id, Name, Quantity__c, Price__c, Condition__c From Merchandise__c Where Condition__c = 'Used'"
read XNAME
force query "Select Id, Name, Quantity__c, Price__c, Condition__c From Merchandise__c Where Condition__c = 'Used'"
