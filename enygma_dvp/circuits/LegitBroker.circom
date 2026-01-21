pragma circom 2.0.0;
include "./templates/LegitBrokerTemplate.circom";

component main {
                public [
                    st_beacon, 
                    st_blindedPublicKey
                ]
            } =  LegitBrokerTemplate();
