# independance-day

The 5/20 export data is 
[https://test1.gno.land/static/cosmos_10562840_export.json](here) (beware, large file).

Please try to replicate this yourself, if you have a full node.
The instructions can be found [./snapshot/cosmoshub_snapshot.md](here). #BOUNTY

## plan

If you can read this, we still need to do the following:

 * distill account balance information from the export json file.
   - the tricky part is in finding the rate between ATOMs and staked "shares",
     and accounting for slashing etc.
 * apply the vote information found here to the above according to weighting
   rules based on voting of proposition 69.

test1.gno.land did not include any reasonable gnot distribution.

test2.gno.land will be the first testnet that includes distribution
information, but it will be incomplete. We will coordinate bounties on
test2.gno.land to complete the distribution for test3.gno.land. 

## contributions

* 0xAN|Nodes.Guru g1jj32fhrz6awxupdw5na244nxutjk99xk847wm2 for 5/20 export
